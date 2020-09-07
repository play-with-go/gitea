// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"os"
	"os/exec"
)

func (r *runner) runSetup(args []string) error {
	adminUser := exec.Command("docker-compose", "exec", "-T", "-u", "git", "gitea", "gitea", "admin", "create-user",
		"--admin", "--username", os.Getenv("PLAYWITHGODEV_ROOT_USER"),
		"--password", os.Getenv("PLAYWITHGODEV_ROOT_PASSWORD"), "--email", "blah@blah.com",
	)
	adminUser.Stdout = os.Stdout
	adminUser.Stderr = os.Stderr
	adminUser.Stdin = os.Stdin
	err := adminUser.Run()
	check(err, "failed to run [%v]: %v", adminUser, err)

	// Now run self in Docker
	org := r.buildSelfDockerCmd([]string{
		"-e", "PLAYWITHGODEV_ROOT_USER",
		"-e", "PLAYWITHGODEV_ROOT_PASSWORD",
		"-e", "PLAYWITHGODEV_GITHUB_USER",
		"-e", "PLAYWITHGODEV_GITHUB_PAT",
	}, "pre")
	err = org.Run()
	check(err, "failed to run [%v]: %v", org, err)

	return nil
}
