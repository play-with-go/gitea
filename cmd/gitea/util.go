// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (r *runner) buildSelfDockerCmd(dockerArgs []string, args ...string) *exec.Cmd {
	var stdout, stderr bytes.Buffer
	rootDirCmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/play-with-go/gitea")
	rootDirCmd.Stdout = &stdout
	rootDirCmd.Stderr = &stderr
	err := rootDirCmd.Run()
	check(err, "failed to run [%v]: %v\n%s", rootDirCmd, err, stderr.Bytes())
	self, err := os.Executable()
	check(err, "failed to determine self: %v", err)
	rootDir := strings.TrimSpace(stdout.String())

	res := exec.Command("docker-compose",
		"-f", filepath.Join(rootDir, "docker-compose.yml"),
		"-f", filepath.Join(rootDir, "docker-compose-playwithgo.yml"),
		"run", "--rm",
		"-v", fmt.Sprintf("%v:/init", self),
		"-v", fmt.Sprintf("%v:/workdir", rootDir), "--workdir=/workdir",
	)
	res.Args = append(res.Args, dockerArgs...)
	res.Args = append(res.Args, "playwithgo", "/init")
	res.Args = append(res.Args, args...)
	res.Stdout = os.Stdout
	res.Stderr = os.Stderr
	res.Stdin = os.Stdin
	res.Env = append(os.Environ(), "GOMODCACHE=")
	// now the caller can add the arguments to the command
	return res
}
