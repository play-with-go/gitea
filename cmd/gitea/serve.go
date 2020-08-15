package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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
	"text/template"
	"time"

	giteasdk "code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v31/github"
	"github.com/play-with-go/gitea"
	"github.com/play-with-go/preguide"
)

func (r *runner) runServe(args []string) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals)

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		raise("failed to get debug build info")
	}
	buildInfoJSON, err := json.MarshalIndent(buildInfo, "", "  ")
	check(err, "failed to JSON marshal build info: %v", err)

	mux := http.NewServeMux()
	mux.HandleFunc("/newuser", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Printf("URL: %v, method: %v\n", req.URL, req.Method)
		if req.URL.Query().Get("get-version") == "1" {
			if req.Method != "GET" {
				resp.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(resp, "GET request required for get-version")
				return
			}
			fmt.Fprintf(resp, "%s", buildInfoJSON)
			return
		}
		if req.Method != "POST" {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(resp, "POST request required for /newuser; URL: %v, method: %v", req.URL, req.Method)
			return
		}
		args := new(gitea.NewUser)
		dec := json.NewDecoder(req.Body)
		if err := dec.Decode(&args); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(resp, "failed to decode request: %v", err)
			return
		}

		res := r.newUser(args)

		enc := json.NewEncoder(resp)
		if err := enc.Encode(res); err != nil {
			// TODO: this header write is probably too late?
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(resp, "failed to encode response: %v", err)
			return
		}
	})

	addr := fmt.Sprintf(":%v", *r.serveCmd.fPort)

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
		signal.Notify(sigint, os.Interrupt)
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

func (r *runner) newUser(args *gitea.NewUser) preguide.PrestepOut {
	// Tidy up old repos
	r.removeOldRepos()
	r.removeOldUsers()

	// User account -> username (gitea)
	user := r.createUser()

	// Create gitea repository in userguides
	repos := r.createUserRepos(user, args.Repos)

	// Add user as a collab on that repo
	r.addUserCollabRepos(repos, user)

	// Add mirroring to repository
	r.addReposMirrorHook(repos)

	// create GitHub repository
	r.createGitHubRepos(repos)

	res := preguide.PrestepOut{
		Vars: []string{
			"GITEA_USERNAME=" + user.UserName,
			"GITEA_PASSWORD=" + user.password,
		},
	}
	for _, repo := range repos {
		res.Vars = append(res.Vars, fmt.Sprintf("%v=%v", repo.repoSpec.Var, repo.Name))
	}
	return res
}

type userPassword struct {
	*giteasdk.User
	password string
}

func (r *runner) removeOldRepos() {
	opt := giteasdk.ListReposOptions{
		ListOptions: giteasdk.ListOptions{
			PageSize: 10,
		},
	}
	now := time.Now()
	for {
		repos, err := r.client.ListUserRepos(UserGuidesRepo, opt)
		check(err, "failed to list repos: %v", err)
		for _, repo := range repos {
			if delta := now.Sub(repo.Created); delta > 3*time.Hour {
				err := r.client.DeleteRepo(UserGuidesRepo, repo.Name)
				check(err, "failed to delete repo %v/%v: %v", UserGuidesRepo, repo.Name, err)
				fmt.Fprintf(os.Stderr, "deleted repo %v/%v (was %v old)\n", UserGuidesRepo, repo.Name, delta)
			}
		}
		if len(repos) < opt.PageSize {
			break
		}
		opt.Page++
	}
}

func (r *runner) removeOldUsers() {
	opt := giteasdk.AdminListUsersOptions{
		ListOptions: giteasdk.ListOptions{
			PageSize: 10,
		},
	}
	now := time.Now()
	for {
		users, err := r.client.AdminListUsers(opt)
		check(err, "failed to list users: %v", err)
		for _, user := range users {
			if delta := now.Sub(user.Created); delta > 3*time.Hour {
				err := r.client.AdminDeleteUser(user.UserName)
				check(err, "failed to delete user %v: %v", user.UserName, err)
				fmt.Fprintf(os.Stderr, "deleted user %v (was %v old)\n", user.UserName, delta)
			}
		}
		if len(users) < opt.PageSize {
			break
		}
		opt.Page++
	}
}

var start = time.Date(2019, time.December, 19, 12, 00, 0, 0, time.UTC)

func (r *runner) genID() string {
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

func (r *runner) createUser() *userPassword {
	var err error
	randBytes := make([]byte, 30)
	n, err := rand.Read(randBytes)
	if n != len(randBytes) || err != nil {
		raise("failed to generate random bytes: got %v, err %v", len(randBytes), err)
	}
	var password bytes.Buffer
	enc := base64.NewEncoder(base64.URLEncoding, &password)
	_, err = enc.Write(randBytes)
	check(err, "failed to base64 encode password: %v", err)
	// Try 3 times... because 3 is a magic number
	for i := 0; i < 3; i++ {
		var user *giteasdk.User
		username := "u" + r.genID()
		no := false
		args := giteasdk.CreateUserOption{
			Username:           username,
			Email:              username + "@play-with-go.dev",
			Password:           password.String(),
			MustChangePassword: &no,
		}
		user, err = r.client.AdminCreateUser(args)
		if err == nil {
			return &userPassword{
				User:     user,
				password: password.String(),
			}
		}
	}
	raise("failed to create user: %v", err)
	return nil
}

func (r *runner) createUserRepos(user *userPassword, repos []gitea.Repo) (res []userRepo) {
repos:
	for _, repoSpec := range repos {
		var err error
		var repo *giteasdk.Repository
		var prefix, suffix string
		if i := strings.LastIndex(repoSpec.Pattern, "*"); i != -1 {
			prefix, suffix = repoSpec.Pattern[:i], repoSpec.Pattern[i+1:]
		} else {
			prefix = repoSpec.Pattern
		}
		for j := 0; j < 3; j++ {
			name := prefix + r.genID() + suffix
			args := giteasdk.CreateRepoOption{
				Name:    name,
				Private: true,
			}
			repo, err = r.client.AdminCreateRepo(UserGuidesRepo, args)
			if err == nil {
				res = append(res, userRepo{
					repoSpec:   repoSpec,
					Repository: repo,
				})
				continue repos
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

func (r *runner) addUserCollabRepos(repos []userRepo, user *userPassword) {
	for _, repo := range repos {
		args := giteasdk.AddCollaboratorOption{}
		err := r.client.AddCollaborator(UserGuidesRepo, repo.Name, user.UserName, args)
		check(err, "failed to add user as collaborator: %v", err)
	}
}

var gitHookTemplate = template.Must(template.New("t").Parse(`
#!/bin/bash
tf=$(mktemp)
trap "rm $tf" EXIT
git push --mirror https://{{.User}}:{{.Password}}@github.com/userguides/{{.Repo}}.git > $tf 2>&1 || { cat $tf && false; }
`[1:]))

func (r *runner) addReposMirrorHook(repos []userRepo) {
	for _, repo := range repos {
		var err error
		var hook bytes.Buffer
		vals := struct {
			User     string
			Password string
			Repo     string
		}{os.Getenv(EnvGithubUser), os.Getenv(EnvGithubPAT), repo.Name}
		err = gitHookTemplate.Execute(&hook, vals)
		check(err, "failed to execute git hook template: %v", err)
		args := giteasdk.EditGitHookOption{
			Content: hook.String(),
		}
		err = r.client.EditRepoGitHook(UserGuidesRepo, repo.Name, "post-receive", args)
		check(err, "failed to edit repo git hook: %v", err)
	}
}

func (r *runner) createGitHubRepos(repos []userRepo) {
	for _, repo := range repos {
		no := false
		desc := fmt.Sprintf("User guide %v", repo.Name)
		_, resp, err := r.github.Repositories.Create(context.Background(), UserGuidesRepo, &github.Repository{
			Name:        &repo.Name,
			Description: &desc,
			HasIssues:   &no,
			HasWiki:     &no,
			HasProjects: &no,
		})
		check(err, "failed to create GitHub repo: %v\n%v", err, resp.Status)
	}
}
