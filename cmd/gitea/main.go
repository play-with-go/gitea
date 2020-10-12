// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

// gitea is a helper to work with a gitea instance
package main

import (
	"os"
)

const (
	EnvRootUser     = "PLAYWITHGODEV_ROOT_USER"
	EnvRootPassword = "PLAYWITHGODEV_ROOT_PASSWORD"

	EnvContributorUser     = "PLAYWITHGODEV_CONTRIBUTOR_USER"
	EnvContributorPassword = "PLAYWITHGODEV_CONTRIBUTOR_PASSWORD"

	GiteaOrg  = "x"
	GitHubOrg = "userguides"

	TemporaryUserFullName = "A really very temporary user"
)

type runner struct {
	*rootCmd
	preCmd            *preCmd
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

	args := r.rootCmd.fs.Args()
	if len(args) == 0 {
		return r.rootCmd.usageErr("missing command")
	}
	cmd := args[0]
	switch cmd {
	case "pre":
		return r.preCmd.run(args[1:])
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
