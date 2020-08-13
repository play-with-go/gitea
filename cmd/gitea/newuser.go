package main

import (
	"io"
	"net/http"
	"os"
)

func (r *runner) runNewUser(args []string) error {
	if err := r.newUserCmd.fs.Parse(args); err != nil {
		return r.newUserCmd.usageErr("failed to parse flags: %v", err)
	}

	// If we are not run with the -docker flag, we need to re-run
	// ourselves in docker passing that flag. That is to say, the -docker
	// flag indicates we are running docker.
	if !*r.newUserCmd.fDocker {
		self := r.buildSelfDockerCmd()
		self.Args = append(self.Args, "newuser", "-docker")
		self.Stdin = os.Stdin
		self.Stdout = os.Stdout
		self.Stderr = os.Stderr
		err := self.Run()
		check(err, "failed to run [%v]: %v", self, err)
		return nil
	}

	u := "http://gitea_prestep:8080/newuser"
	// We are running in the relevant docker envionment. Make an HTTP request
	// using stdin as the body
	resp, err := http.Post(u, "application/json", os.Stdin)
	check(err, "failed to post to %v: %v", u, err)

	defer resp.Body.Close()
	_, err = io.Copy(os.Stdout, resp.Body)
	check(err, "failed to read response from %v: %v", u, err)

	return nil
}
