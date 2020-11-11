// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	giteasdk "code.gitea.io/sdk/gitea"
	"github.com/play-with-go/gitea"
	"github.com/play-with-go/preguide"
	"gopkg.in/retry.v1"
)

func (sc *serveCmd) run(args []string) error {
	if len(args) > 0 {
		raise("serve does not take any flags or arguments")
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals)

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		raise("failed to get debug build info")
	}
	buildInfoJSON, err := json.MarshalIndent(buildInfo, "", "  ")
	check(err, "failed to JSON marshal build info: %v", err)

	clientCreate := make(chan int)
	go func() {
		strategy := retry.LimitTime(5*time.Second,
			retry.Exponential{
				Initial: 100 * time.Millisecond,
				Factor:  1.5,
			},
		)
		for a := retry.Start(strategy, nil); a.Next(); {
			fmt.Printf("Connecting to %v\n", *sc.fRootURL)
			sc.client, err = giteasdk.NewClient(*sc.fRootURL)
			if err == nil {
				break
			}
		}
		check(err, "failed to create root client: %v", err)
		sc.client.SetBasicAuth(os.Getenv(EnvContributorUser), os.Getenv(EnvContributorPassword))
		close(clientCreate)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("get-version") != "1" || req.Method != "GET" {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprintf(resp, "%s", buildInfoJSON)
	})
	mux.HandleFunc("/newuser", func(resp http.ResponseWriter, req *http.Request) {
		<-clientCreate
		// Requires contriburo credentials
		if req.Method != "POST" {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		args := new(gitea.NewUser)
		dec := json.NewDecoder(req.Body)
		if err := dec.Decode(&args); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(resp, "failed to decode request: %v", err)
			return
		}

		res := sc.newUser(args)

		enc := json.NewEncoder(resp)
		if err := enc.Encode(res); err != nil {
			// TODO: this header write is probably too late?
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(resp, "failed to encode response: %v", err)
			return
		}
	})

	addr := fmt.Sprintf(":%v", *sc.fPort)

	srv := &http.Server{
		Handler: mux,
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	errors := make(chan error)
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		if err := srv.Shutdown(context.Background()); err != nil {
			errors <- err
		}
		close(errors)
	}()
	fmt.Fprintf(os.Stderr, "Listening on %v\n", addr)
	if err := srv.Serve(l); err != http.ErrServerClosed {
		raise("HTTP server failed: %v", err)
	}
	if err := <-errors; err != nil {
		raise("HTTP server shutdown failed: %v", err)
	}
	return nil
}

func (sc *serveCmd) newUser(args *gitea.NewUser) preguide.PrestepOut {
	// User account -> username (gitea)
	user := sc.createUser()

	// Create gitea repositories in userguides
	repos := sc.createUserRepos(user, args.Repos)

	res := preguide.PrestepOut{
		Vars: []string{
			"GITEA_USERNAME=" + user.UserName,
			"GITEA_PASSWORD=" + user.password,
		},
	}
	for _, repo := range repos {
		res.Vars = append(res.Vars, fmt.Sprintf("%v=%v/%v/%v", repo.repoSpec.Var, sc.hostname, user.UserName, repo.Name))
	}
	return res
}

type userPassword struct {
	*giteasdk.User
	password string
}

var start = time.Date(2019, time.December, 19, 12, 00, 0, 0, time.UTC)

func (sc *serveCmd) genID() string {
	now := time.Now()
	diff := (now.UnixNano() - start.UnixNano()) / 1000000
	bs := make([]byte, binary.MaxVarintLen64)
	n := binary.PutVarint(bs, diff)
	var buf bytes.Buffer
	enc := hex.NewEncoder(&buf)
	if _, err := enc.Write(bs[:n]); err != nil {
		panic(err)
	}
	id := buf.String()
	id = strings.ToLower(id)
	return id
}

func (sc *serveCmd) createUser() *userPassword {
	var err error
	password := randomPassword()
	// Try 3 times... because 3 is a magic number
	for i := 0; i < 3; i++ {
		var user *giteasdk.User
		username := "u" + sc.genID()
		no := false
		zero := 0
		args := giteasdk.CreateUserOption{
			FullName:           TemporaryUserFullName,
			Username:           username,
			Email:              fmt.Sprintf("%v@%v", username, sc.hostname),
			Password:           password,
			MustChangePassword: &no,
		}
		user, _, err = sc.client.AdminCreateUser(args)
		if err != nil {
			continue
		}
		_, err = sc.client.AdminEditUser(user.UserName, giteasdk.EditUserOption{
			Email:                   user.Email,
			FullName:                user.FullName,
			MaxRepoCreation:         &zero,
			AllowCreateOrganization: &no,
			AllowGitHook:            &no,
		})
		check(err, "failed to edit user %v: %v", user.UserName, err)

		return &userPassword{
			User:     user,
			password: password,
		}
	}
	raise("failed to create user: %v", err)
	return nil
}

func (sc *serveCmd) createUserRepos(user *userPassword, repos []gitea.Repo) (res []userRepo) {
repos:
	for _, repoSpec := range repos {
		var err error
		var repo *giteasdk.Repository
		var prefix, suffix string
		var hasRandomPart bool
		if i := strings.LastIndex(repoSpec.Pattern, "*"); i != -1 {
			hasRandomPart = true
			prefix, suffix = repoSpec.Pattern[:i], repoSpec.Pattern[i+1:]
		} else {
			prefix = repoSpec.Pattern
		}
		for j := 0; j < 3; j++ {
			name := prefix
			if hasRandomPart {
				name += sc.genID() + suffix
			}
			args := giteasdk.CreateRepoOption{
				Name:    name,
				Private: repoSpec.Private,
			}
			repo, _, err = sc.client.AdminCreateRepo(user.UserName, args)
			if err == nil {
				res = append(res, userRepo{
					repoSpec:   repoSpec,
					Repository: repo,
				})
				continue repos
			}
			if !hasRandomPart {
				break
			}
		}
		raise("failed to create user repostitory: %v", err)
	}
	return
}

type userRepo struct {
	repoSpec gitea.Repo
	*giteasdk.Repository
}
