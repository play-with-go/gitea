// gitea is a helper to work with a gitea instance
package main

import (
	"os"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/v31/github"
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
	setupCmd   *setupCmd
	preCmd     *preCmd
	serveCmd   *serveCmd
	newUserCmd *newUserCmd

	client *gitea.Client

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
	case "setup":
		return r.runSetup(args[1:])
	case "pre":
		return r.runPre(args[1:])
	case "serve":
		return r.runServe(args[1:])
	case "newuser":
		return r.runNewUser(args[1:])
	default:
		return r.rootCmd.usageErr("unknown command: " + cmd)
	}
}
