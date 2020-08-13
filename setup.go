package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (r *runner) runSetup(args []string) error {
	if err := r.preCmd.fs.Parse(args); err != nil {
		return r.preCmd.usageErr("failed to parse flags: %v", err)
	}
	migrate := exec.Command("docker-compose", "exec", "-T", "-u", "git", "gitea", "gitea", "migrate")
	out, err := migrate.CombinedOutput()
	check(err, "failed to run [%v]: %v\n%s", migrate, err, out)

	adminUser := exec.Command("docker-compose", "exec", "-T", "-u", "git", "gitea", "gitea", "admin", "create-user",
		"--admin", "--username", os.Getenv("PLAYWITHGODEV_ROOT_USER"),
		"--password", os.Getenv("PLAYWITHGODEV_ROOT_PASSWORD"), "--email", "blah@blah.com",
	)
	out, err = adminUser.CombinedOutput()
	check(err, "failed to run [%v]: %v\n%s", adminUser, err, out)

	var stdout, stderr bytes.Buffer
	rootDirCmd := exec.Command("go", "list", "-f", "{{.Dir}}", "github.com/play-with-go/gitea")
	rootDirCmd.Stdout = &stdout
	rootDirCmd.Stderr = &stderr
	err = rootDirCmd.Run()
	check(err, "failed to run [%v]: %v\n%s", rootDirCmd, err, stderr.Bytes())

	self, err := os.Executable()
	check(err, "failed to determine self: %v", err)

	rootDir := strings.TrimSpace(stdout.String())

	// Now run self in Docker
	org := exec.Command("docker-compose",
		"-f", filepath.Join(rootDir, "docker-compose.yml"),
		"-f", filepath.Join(rootDir, "docker-compose-playwithgo.yml"),
		"run",
		"-v", fmt.Sprintf("%v:/init", self), "--entrypoint=/init",
		"-v", fmt.Sprintf("%v:/workdir", rootDir), "--workdir=/workdir",
		"-e", "PLAYWITHGODEV_ROOT_USER",
		"-e", "PLAYWITHGODEV_ROOT_PASSWORD",
		"-e", "PLAYWITHGODEV_GITHUB_USER",
		"-e", "PLAYWITHGODEV_GITHUB_PAT",
		"playwithgo", "pre",
	)
	out, err = org.CombinedOutput()
	check(err, "failed to run [%v]: %v\n%s", org, err, out)

	return nil
}
