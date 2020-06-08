// gitea is a helper to work with a gitea instance
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	mathrand "math/rand"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v31/github"
	"golang.org/x/crypto/ssh"
	"gopkg.in/retry.v1"
)

const (
	EnvRootUser     = "PLAYWITHGODEV_ROOT_USER"
	EnvRootPassword = "PLAYWITHGODEV_ROOT_PASSWORD"

	EnvGithubUser = "PLAYWITHGODEV_GITHUB_USER"
	EnvGithubPAT  = "PLAYWITHGODEV_GITHUB_PAT"

	UserGuidesRepo = "userguides"
)

type runner struct {
	*rootCmd
	waitCmd    *waitCmd
	preCmd     *preCmd
	newUserCmd *newUserCmd
	client     *gitea.Client

	github *github.Client
}

func newRunner() *runner {
	return &runner{}
}

func (r *runner) mainerr() (err error) {
	defer handleKnown(&err)

	if err := r.rootCmd.fs.Parse(os.Args[1:]); err != nil {
		return usageErr{err, r.rootCmd}
	}

	r.client = gitea.NewClient(*r.fRootURL, "")
	r.client.SetBasicAuth(os.Getenv(EnvRootUser), os.Getenv(EnvRootPassword))

	auth := github.BasicAuthTransport{
		Username: os.Getenv(EnvGithubUser),
		Password: os.Getenv(EnvGithubPAT),
	}
	r.github = github.NewClient(auth.Client())

	args := r.rootCmd.fs.Args()
	if len(args) == 0 {
		return r.rootCmd.usageErr("missing command")
	}
	cmd := args[0]
	switch cmd {
	case "wait":
		return r.runWait(args[1:])
	case "pre":
		return r.runPre(args[1:])
	case "newuser":
		return r.runNewUser(args[1:])
	default:
		return r.rootCmd.usageErr("unknown command: " + cmd)
	}
}

func (r *runner) runWait(args []string) error {
	if err := r.waitCmd.fs.Parse(args); err != nil {
		return r.waitCmd.usageErr("failed to parse flags: %v", err)
	}
	// Try the version endpoint with backoff until success or timeout
	wait, err := time.ParseDuration(*r.waitCmd.fWait)
	check(err, "failed to parse duration from -wait flag: %v", err)

	strategy := retry.LimitTime(wait,
		retry.Exponential{
			Initial: 10 * time.Millisecond,
			Factor:  1.5,
		},
	)
	for a := retry.Start(strategy, nil); a.Next(); {
		_, err = r.client.ServerVersion()
		if err == nil {
			break
		}
	}
	return err
}

func (r *runner) runPre(args []string) error {
	if err := r.preCmd.fs.Parse(args); err != nil {
		return r.preCmd.usageErr("failed to parse flags: %v", err)
	}
	var err error
	_, err = r.client.CreateOrg(gitea.CreateOrgOption{
		UserName:   UserGuidesRepo,
		Visibility: "private",
	})
	check(err, "failed to create %v organisation: %v", UserGuidesRepo, err)
	return nil
}

var bashTemplate = template.Must(template.New("bashTemplate").Parse(`
#!/usr/bin/env bash
umask 0077
cd ~/
mkdir .ssh
cat <<EOD > ~/.ssh/id_ed25519
{{.PrivKey}}
EOD
cat <<EOD > ~/.ssh/id_ed25519.pub
{{.PubKey}}
EOD
eval $(ssh-agent)
ssh-add
cd $(mktemp -d)
cat <<EOD > ~/.ssh/known_hosts
{{.KeyScan}}
EOD
cat <<EOD > ~/.gitconfig
[user]
  name = {{.Username}}
  email = {{.Username}}@play-with-go.dev
EOD
cat <<EOD > ~/.netrc
machine play-with-go.dev login {{.GiteaUsername}} password {{.Password}}
EOD
`[1:]))

var testTemplate = template.Must(template.New("bashTemplate").Parse(`
git clone https://play-with-go.dev/userguides/{{index .Repos 0}}
cd {{index .Repos 0}}
cat <<EOD > README.md
This is a test
EOD
git add -A
git commit -am 'Initial commit'
git push
`[1:]))

type userPassword struct {
	*gitea.User
	password string
}

func (r *runner) runNewUser(args []string) error {
	if err := r.newUserCmd.fs.Parse(args); err != nil {
		return r.newUserCmd.usageErr("failed to parse flags: %v", err)
	}
	args = r.newUserCmd.fs.Args()
	var mode string
	switch len(args) {
	case 0:
		mode = "out"
	case 1:
		mode = args[0]
	default:
		return r.newUserCmd.usageErr("too many arguments; expected at most 1")
	}
	switch mode {
	case "out":
		if *r.newUserCmd.fTest {
			raise("-test can only be supplied in raw mode")
		}
	case "raw":
	default:
		return r.newUserCmd.usageErr("unknown command %v", mode)
	}
	// Tidy up old repos
	r.removeOldRepos()
	r.removeOldUsers()

	// User account -> username (gitea)
	user := r.createUser()

	priv, pub := r.createUserSSHKey()

	// ssh-key (upload to gitea)
	r.setUserSSHKey(user, pub)

	// Create gitea repository in userguides
	repos := r.createUserRepos(user, *r.newUserCmd.fNumRepos)

	// Add user as a collab on that repo
	r.addUserCollabRepos(repos, user)

	// Add mirroring to repository
	r.addReposMirrorHook(repos)

	// create GitHub repository
	r.createGitHubRepos(repos)

	// Scan ssh host and trust
	keyScan := r.keyScan()

	vals := struct {
		PrivKey       string
		PubKey        string
		KeyScan       string
		Username      string
		GiteaUsername string
		Password      string
		Repos         []string
	}{priv, pub, keyScan, *r.newUserCmd.fUsername, user.UserName, user.password, nil}

	for _, repo := range repos {
		vals.Repos = append(vals.Repos, repo.Name)
	}

	var script bytes.Buffer
	err := bashTemplate.Execute(&script, vals)
	check(err, "failed to execute bash template: %v", err)

	switch mode {
	case "raw":
		if *r.newUserCmd.fTest {
			err = testTemplate.Execute(&script, vals)
		}
		check(err, "failed to executed test template: %v", err)
		fmt.Print(script.String())
		return nil
	case "out":
	default:
		raise("unknown mode %v", mode)
	}

	var out struct {
		Script string
		Vars   map[string]string
	}
	out.Script = script.String()
	out.Vars = map[string]string{
		"$USER": user.UserName,
	}
	if len(repos) == 1 {
		out.Vars["$REPO"] = repos[0].Name
	} else {
		for i, repo := range repos {
			out.Vars[fmt.Sprintf("$REPO%v", i+1)] = repo.Name
		}
	}
	enc := json.NewEncoder(os.Stdout)
	err = enc.Encode(out)
	check(err, "failed to encode output: %v", err)

	return nil
}

func (r *runner) removeOldRepos() {
	opt := gitea.ListReposOptions{
		ListOptions: gitea.ListOptions{
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
	opt := gitea.AdminListUsersOptions{
		ListOptions: gitea.ListOptions{
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
	enc := base32.NewEncoder(base32.HexEncoding, &buf)
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
		var user *gitea.User
		username := "u" + r.genID()
		no := false
		args := gitea.CreateUserOption{
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

func (r *runner) createUserSSHKey() (string, string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	check(err, "failed to generate ed25519 key: %v", err)
	privKey := pem.EncodeToMemory(&pem.Block{Type: "OPENSSH PRIVATE KEY", Bytes: marshalED25519PrivateKey(priv, pub)})
	check(err, "failed to pem-encode private key: %v", err)
	pubKey, err := ssh.NewPublicKey(pub)
	check(err, "failed to create public key: %v", err)
	authKey := ssh.MarshalAuthorizedKey(pubKey)
	return string(privKey), string(authKey)
}

func (r *runner) setUserSSHKey(user *userPassword, pub string) {
	args := gitea.CreateKeyOption{
		Title:    "ssh key",
		Key:      pub,
		ReadOnly: false,
	}
	_, err := r.client.AdminCreateUserPublicKey(user.UserName, args)
	check(err, "failed to set user SSH key: %v", err)
}

func (r *runner) createUserRepos(user *userPassword, n int) (res []*gitea.Repository) {
repos:
	for i := 0; i < n; i++ {
		var err error
		var repo *gitea.Repository
		for j := 0; j < 3; j++ {
			name := "r" + r.genID()
			args := gitea.CreateRepoOption{
				Name:    name,
				Private: true,
			}
			repo, err = r.client.AdminCreateRepo(UserGuidesRepo, args)
			if err == nil {
				res = append(res, repo)
				continue repos
			}
		}
		raise("failed to create user repostitory: %v", err)
	}
	return
}

func (r *runner) addUserCollabRepos(repos []*gitea.Repository, user *userPassword) {
	for _, repo := range repos {
		args := gitea.AddCollaboratorOption{}
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

func (r *runner) addReposMirrorHook(repos []*gitea.Repository) {
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
		args := gitea.EditGitHookOption{
			Content: hook.String(),
		}
		err = r.client.EditRepoGitHook(UserGuidesRepo, repo.Name, "post-receive", args)
		check(err, "failed to edit repo git hook: %v", err)
	}
}

func (r *runner) createGitHubRepos(repos []*gitea.Repository) {
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

func (r *runner) keyScan() string {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("ssh-keyscan", "-H", "play-with-go.dev")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	check(err, "failed to run [%v]: %v\n%s", strings.Join(cmd.Args, " "), err, stderr.Bytes())
	return strings.TrimSpace(stdout.String())
}

func (r *runner) debugf(format string, args ...interface{}) {
	if *r.fDebug {
		fmt.Fprintf(os.Stderr, format, args...)
	}
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
