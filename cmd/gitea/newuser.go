package main

import (
	"bytes"
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
		self := r.buildSelfDockerCmd(nil, "newuser", "-docker")
		err := self.Run()
		check(err, "failed to run [%v]: %v", self, err)
		return nil
	}

	checkStatus := func(resp *http.Response, format string, args ...interface{}) {
		if resp.StatusCode/100 != 2 {
			defer resp.Body.Close()
			var body bytes.Buffer
			_, err := io.Copy(&body, resp.Body)
			check(err, "failed to read error response body: %v", err)
			args = append(args, body.String())
			raise(format, args...)
		}
	}

	rootURL := "http://gitea_prestep:8080"
	versionResp, err := http.Get(rootURL + "?get-version=1")
	check(err, "failed to get version information from %v: %v", rootURL, err)
	checkStatus(versionResp, "get version request from %v not successful: %v", rootURL)

	defer versionResp.Body.Close()
	_, err = io.Copy(os.Stdout, versionResp.Body)
	check(err, "failed to read version response: %v", err)

	// We are running in the relevant docker envionment. Make an HTTP request
	// using stdin as the body
	newuserURL := rootURL + "/newuser"
	newuser, err := http.Post(newuserURL, "application/json", os.Stdin)
	check(err, "failed to post to %v: %v", newuserURL, err)
	checkStatus(newuser, "newuser request not successful: %v")

	defer newuser.Body.Close()
	_, err = io.Copy(os.Stdout, newuser.Body)
	check(err, "failed to read newuser main response: %v", err)

	return nil
}
