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
	out, err := migrate.CombinedOutput()
	check(err, "failed to run [%v]: %v\n%s", migrate, err, out)

	adminUser := exec.Command("docker-compose", "exec", "-T", "-u", "git", "gitea", "gitea", "admin", "create-user",
		"--admin", "--username", os.Getenv("PLAYWITHGODEV_ROOT_USER"),
		"--password", os.Getenv("PLAYWITHGODEV_ROOT_PASSWORD"), "--email", "blah@blah.com",
	)
	out, err = adminUser.CombinedOutput()
	check(err, "failed to run [%v]: %v\n%s", adminUser, err, out)

	// Now run self in Docker
	org := r.buildSelfDockerCmd(
		"-e", "PLAYWITHGODEV_ROOT_USER",
		"-e", "PLAYWITHGODEV_ROOT_PASSWORD",
		"-e", "PLAYWITHGODEV_GITHUB_USER",
		"-e", "PLAYWITHGODEV_GITHUB_PAT",
	)
	// Add the args to self
	org.Args = append(org.Args, "pre")
	out, err = org.CombinedOutput()
	check(err, "failed to run [%v]: %v\n%s", org, err, out)

	return nil
}
