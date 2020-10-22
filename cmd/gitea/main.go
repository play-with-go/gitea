// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

// gitea is a helper to work with a gitea instance
package main

import (
	"net/url"
	"os"
)

//go:generate go run cuelang.org/go/cmd/cue cmd genimagebases

const (
	EnvRootUser     = "PLAYWITHGODEV_ROOT_USER"
	EnvRootPassword = "PLAYWITHGODEV_ROOT_PASSWORD"

	EnvContributorUser     = "PLAYWITHGODEV_CONTRIBUTOR_USER"
	EnvContributorPassword = "PLAYWITHGODEV_CONTRIBUTOR_PASSWORD"

	TemporaryUserFullName = "A really very temporary user"
)

type runner struct {
	*rootCmd
	serveCmd          *serveCmd
	newContributorCmd *newContributorCmd
	reapCmd           *reapCmd
}

func newRunner() *runner {
	return &runner{}
}

func (r *runner) mainerr() (err error) {
	defer handleKnown(&err)

	if err := r.rootCmd.fs.Parse(os.Args[1:]); err != nil {
		return usageErr{err, r.rootCmd}
	}

	u, err := url.Parse(*r.fRootURL)
	check(err, "failed to parse -rootURL value %q: %v", *r.fRootURL, err)
	r.rootCmd.hostname = u.Hostname()

	args := r.rootCmd.fs.Args()
	if len(args) == 0 {
		return r.rootCmd.usageErr("missing command")
	}
	cmd := args[0]
	switch cmd {
	case "serve":
		return r.serveCmd.run(args[1:])
	case "reap":
		return r.reapCmd.run(args[1:])
	case "newcontributor":
		return r.newContributorCmd.run(args[1:])
	default:
		return r.rootCmd.usageErr("unknown command: " + cmd)
	}
}
