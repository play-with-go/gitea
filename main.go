// preguide is a pre-processor for Play With Docker-based guides
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"net"
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
		UserName: UserGuidesRepo,
	})
	check(err, "failed to create %v organisation: %v", UserGuidesRepo, err)
	return nil
}

func (r *runner) runNewUser(args []string) error {
	if err := r.newUserCmd.fs.Parse(args); err != nil {
		return r.newUserCmd.usageErr("failed to parse flags: %v", err)
	}
	// Get IP address of play-with-go host
	// ip := r.getIPOfPlayWithGo()

	// User account -> username (gitea)
	user := r.createUser()

	priv, pub := r.createUserSSHKey()

	// ssh-key (upload to gitea)
	r.setUserSSHKey(user, pub)

	// Create gitea repository in userguides
	repo := r.createUserRepo(user)

	// Add user as a collab on that repo
	r.addUserCollabRepo(repo, user)

	// Add mirroring to repository
	r.addRepoMirrorHook(repo)

	// create GitHub repository
	r.createGitHubRepo(user)

	// Scan ssh host and trust
	keyScan := r.keyScan()

	t := template.Must(template.New("out").Parse(`
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
git clone ssh://git@play-with-go.dev/userguides/{{.Username}}.git
cd {{.Username}}
cat <<EOD > README
This is a test
EOD
git add -A
git commit -am 'Initial commit'
`[1:]))
	vals := struct {
		PrivKey  string
		PubKey   string
		KeyScan  string
		Username string
	}{priv, pub, keyScan, user.UserName}

	err := t.Execute(os.Stdout, vals)
	check(err, "failed to execute template: %v", err)

	return nil
}

func (r *runner) getIPOfPlayWithGo() string {
	addr, err := net.LookupIP("play-with-go.dev")
	check(err, "failed to lookup IP address of play-with-go.dev: %v", err)
	if len(addr) != 1 {
		raise("found %v IP addresses for play-with-go.dev; expected 1", len(addr))
	}
	return addr[0].String()
}

func (r *runner) createUser() *gitea.User {
	// Try 3 times... because 3 is a magic number
	var err error
	start := time.Date(2019, time.December, 19, 12, 00, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		now := time.Now()
		diff := (now.UnixNano() - start.UnixNano()) / 1000
		bs := make([]byte, binary.MaxVarintLen64)
		n := binary.PutVarint(bs, diff)
		var buf bytes.Buffer
		enc := base64.NewEncoder(base64.URLEncoding, &buf)
		if _, err := enc.Write(bs[:n]); err != nil {
			panic(err)
		}
		username := fmt.Sprintf("user%v", buf.String())
		args := gitea.CreateUserOption{
			Username: username,
			Email:    username + "@play-with-go.dev",
			Password: username,
		}
		user, err := r.client.AdminCreateUser(args)
		if err == nil {
			return user
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

func (r *runner) setUserSSHKey(user *gitea.User, pub string) {
	args := gitea.CreateKeyOption{
		Title:    "ssh key",
		Key:      pub,
		ReadOnly: false,
	}
	_, err := r.client.AdminCreateUserPublicKey(user.UserName, args)
	check(err, "failed to set user SSH key: %v", err)
}

func (r *runner) createUserRepo(user *gitea.User) *gitea.Repository {
	args := gitea.CreateRepoOption{
		Name:    user.UserName,
		Private: true,
	}
	repo, err := r.client.AdminCreateRepo(UserGuidesRepo, args)
	check(err, "failed to create user repostitory %v/%v: %v", UserGuidesRepo, user.UserName, err)
	return repo
}

func (r *runner) addUserCollabRepo(repo *gitea.Repository, user *gitea.User) {
	args := gitea.AddCollaboratorOption{}
	err := r.client.AddCollaborator(UserGuidesRepo, repo.Name, user.UserName, args)
	check(err, "failed to add user as collaborator: %v", err)
}

func (r *runner) addRepoMirrorHook(repo *gitea.Repository) {
	args := gitea.EditGitHookOption{
		Content: fmt.Sprintf("#!/bin/bash\ngit push --mirror https://%v:%v@github.com/userguides/%v.git", os.Getenv(EnvGithubUser), os.Getenv(EnvGithubPAT), repo.Name),
	}
	err := r.client.EditRepoGitHook(UserGuidesRepo, repo.Name, "post-receive", args)
	check(err, "failed to edit repo git hook: %v", err)
}

func (r *runner) createGitHubRepo(user *gitea.User) {
	no := false
	_, resp, err := r.github.Repositories.Create(context.Background(), UserGuidesRepo, &github.Repository{
		Name:        &user.UserName,
		HasIssues:   &no,
		HasWiki:     &no,
		HasProjects: &no,
	})
	check(err, "failed to create GitHub repo: %v\n%v", err, resp.Status)
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
