// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	mathrand "math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	giteasdk "code.gitea.io/sdk/gitea"
	"github.com/play-with-go/gitea"
	"github.com/play-with-go/preguide"
	"golang.org/x/crypto/ssh"
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

	keyScanComplete := make(chan int)
	go func() {
		sc.runKeyScan()
		close(keyScanComplete)
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
		<-keyScanComplete
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

	priv, pub := sc.createUserSSHKey()

	// ssh-key (upload to gitea)
	sc.setUserSSHKey(user, pub)

	// Create gitea repositories in userguides
	repos := sc.createUserRepos(user, args.Repos)

	res := preguide.PrestepOut{
		Vars: []string{
			"GITEA_USERNAME=" + user.UserName,
			"GITEA_PRIV_KEY=" + priv,
			"GITEA_PUB_KEY=" + pub,
			"GITEA_KEYSCAN=" + sc.keyScan,
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
			LoginName:               user.UserName,
			Email:                   &user.Email,
			FullName:                &user.FullName,
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

func (sc *serveCmd) createUserSSHKey() (string, string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	check(err, "failed to generate ed25519 key: %v", err)
	privKey := pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: marshalED25519PrivateKey(priv, pub)})
	check(err, "failed to pem-encode private key: %v", err)
	pubKey, err := ssh.NewPublicKey(pub)
	check(err, "failed to create public key: %v", err)
	authKey := ssh.MarshalAuthorizedKey(pubKey)
	return string(privKey), string(authKey)
}

func (sc *serveCmd) setUserSSHKey(user *userPassword, pub string) {
	args := giteasdk.CreateKeyOption{
		Title:    "ssh key",
		Key:      pub,
		ReadOnly: false,
	}
	_, _, err := sc.client.AdminCreateUserPublicKey(user.UserName, args)
	check(err, "failed to set user SSH key: %v", err)
}

func (sc *serveCmd) runKeyScan() {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ssh-keyscan", "-H", "gopher.live")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	check(err, "failed to run [%v]: %v\n%s", strings.Join(cmd.Args, " "), err, stderr.Bytes())
	sc.keyScan = strings.TrimSpace(stdout.String())
}

// marshalED25519PrivateKey is based on https://github.com/mikesmitty/edkey
func marshalED25519PrivateKey(priv ed25519.PrivateKey, pub ed25519.PublicKey) []byte {
	magic := append([]byte("openssh-key-v1"), 0)

	var w struct {
		CipherName   string
		KdfName      string
		KdfOpts      string
		NumKeys      uint32
		PubKey       []byte
		PrivKeyBlock []byte
	}

	// Fill out the private key fields
	var pk1 struct {
		Check1  uint32
		Check2  uint32
		Keytype string
		Pub     []byte
		Priv    []byte
		Comment string
		Pad     []byte `ssh:"rest"`
	}

	// Set our check ints
	ci := mathrand.Uint32()
	pk1.Check1 = ci
	pk1.Check2 = ci

	// Set our key type
	pk1.Keytype = ssh.KeyAlgoED25519

	// Add the pubkey to the optionally-encrypted block
	pubKey := []byte(pub)
	pk1.Pub = pubKey

	// Add our private key
	pk1.Priv = []byte(priv)

	// Might be useful to put something in here at some point
	pk1.Comment = ""

	// Add some padding to match the encryption block size within PrivKeyBlock (without Pad field)
	// 8 doesn't match the documentation, but that's what ssh-keygen uses for unencrypted keys. *shrug*
	bs := 8
	blockLen := len(ssh.Marshal(pk1))
	padLen := (bs - (blockLen % bs)) % bs
	pk1.Pad = make([]byte, padLen)

	// Padding is a sequence of bytes like: 1, 2, 3...
	for i := 0; i < padLen; i++ {
		pk1.Pad[i] = byte(i + 1)
	}

	// Generate the pubkey prefix "\0\0\0\nssh-ed25519\0\0\0 "
	prefix := []byte{0x0, 0x0, 0x0, 0x0b}
	prefix = append(prefix, []byte(ssh.KeyAlgoED25519)...)
	prefix = append(prefix, []byte{0x0, 0x0, 0x0, 0x20}...)

	// Only going to support unencrypted keys for now
	w.CipherName = "none"
	w.KdfName = "none"
	w.KdfOpts = ""
	w.NumKeys = 1
	w.PubKey = append(prefix, pubKey...)
	w.PrivKeyBlock = ssh.Marshal(pk1)

	magic = append(magic, ssh.Marshal(w)...)

	return magic
}
