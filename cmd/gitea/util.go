// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"os"
	"os/exec"
)

func (r *runner) buildSelfDockerCmd(dockerArgs []string, args ...string) *exec.Cmd {
	res := exec.Command("docker-compose",
		"run", "--rm",
	)
	res.Args = append(res.Args, dockerArgs...)
	res.Args = append(res.Args, "cmd_gitea")
	res.Args = append(res.Args, args...)
	res.Stdout = os.Stdout
	res.Stderr = os.Stderr
	res.Stdin = os.Stdin
	res.Env = append(os.Environ(), "GOMODCACHE=")
	// now the caller can add the arguments to the command
	return res
}
