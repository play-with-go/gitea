package main

import (
	"os"
	"os/exec"
)

func (r *runner) runSetup(args []string) error {
	if err := r.setupCmd.fs.Parse(args); err != nil {
		return r.setupCmd.usageErr("failed to parse flags: %v", err)
	}
	migrate := exec.Command("docker-compose", "exec", "-T", "-u", "git", "gitea", "gitea", "migrate")
	migrate.Stdout = os.Stdout
	migrate.Stderr = os.Stderr
	migrate.Stdin = os.Stdin
	err := migrate.Run()
	check(err, "failed to run [%v]: %v", migrate, err)

	adminUser := exec.Command("docker-compose", "exec", "-T", "-u", "git", "gitea", "gitea", "admin", "create-user",
		"--admin", "--username", os.Getenv("PLAYWITHGODEV_ROOT_USER"),
		"--password", os.Getenv("PLAYWITHGODEV_ROOT_PASSWORD"), "--email", "blah@blah.com",
	)
	adminUser.Stdout = os.Stdout
	adminUser.Stderr = os.Stderr
	adminUser.Stdin = os.Stdin
	err = adminUser.Run()
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
