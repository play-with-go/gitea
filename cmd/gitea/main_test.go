// Copyright 2020 The play-with-go.dev Authors. All rights reserved.  Use of
// this source code is governed by a BSD-style license that can be found in the
// LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/kr/pretty"
	"github.com/play-with-go/preguide"
	"gopkg.in/retry.v1"
)

// TestEverything tests, well, everything. It is assumed we are running the test in an
// environment where we have root credentials available. The process is roughly:
//
// 1.  Start the docker-compose setup
// 2.  Migrate
// 3.  Create root user
// 4.  Run cmd/gitea pre to create x org
// 5.  Run cmd/gitea newcontributor to create a new contributor, extracting the token
//     created as part of the setup
// 6.  Stop the docker-compose setup
// 7.  Restart the docker-compose setup, this time with the contributor credentials set
//     so that the cmd_gitea prestep can use them
// 8.  Call the cmd_gitea prestep to create a new user + repo
// 9.  Decode to make sure we got the expected env vars
// 10. Reap the user we just created
//
// This whole setup is rather complicated by the fact that everything needs to
// be run within docker-compose, but the test itself is invoked from outside
// that setup. Therefore we build and then run the setup within an "isolated"
// COMPOSE_PROJECT_NAME-driven setup.
//
// A number of the steps above require us to invoke cmd/gitea. This could
// easily be achieved by docker-compose run-ing a separate instance of
// cmd_gitea (which will have been built by this point). However, we need to
// run some arbitrary http client code as part of step 8, so it's somewhat
// cleaner to run "self" (i.e. the test binary that includes TestEverything)
// and use plain docker run. This means we keep the docker-compose setup
// "clean", as well as not adding extraneous commands for testing purposes to
// cmd/gitea.
//
// This running of "self" is driven by the GITEA_COMMAND environment variable.
// The controlling process is TestEverything. It initiates all the steps above,
// as well as the invoking of "self". Inovking "self" takes two different
// forms:
//
// 1. calling the prestep calling code - the running of part N from this test
//    binary; functions listed below
// 2. self - the running of main1 in cmd/gitea

func TestMain(m *testing.M) {
	switch c := os.Getenv("GITEA_COMMAND"); c {
	case "prestep", "wait":
		os.Exit(runTestStep(c))
	case "self":
		os.Exit(main1())
	case "":
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %v", c)
		os.Exit(1)
	}
	// call flag.Parse() here if TestMain uses flags
	os.Exit(m.Run())
}

type testRunner struct {
	*testing.T
	root                  string
	selfPath              string
	envComposeProjectName string
}

func newTestRunner(t *testing.T) *testRunner {
	tr := &testRunner{
		T:                     t,
		envComposeProjectName: fmt.Sprintf("gitea_test%v", time.Now().UnixNano()),
	}
	listOut, _ := tr.mustRun(exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/play-with-go/gitea"))
	tr.root = strings.TrimSpace(string(listOut))
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to derive self: %v", err)
	}
	tr.selfPath = self
	return tr
}

func TestEverything(t *testing.T) {
	// We need to run ourself in a
	if runtime.GOOS != "linux" {
		t.Fatal("Can only run this test on linux (left as a fatal error to ensure we get some coverage)")
	}
	tr := newTestRunner(t)

	// Ensure it's torn down at the end of the test
	t.Cleanup(func() {
		tr.mustRunDockerCompose("down", "-t", "0", "-v")
	})

	// Build the docker-compose setup
	tr.mustRunDockerCompose("build")

	// Start the docker-compose instance in the background
	tr.mustRunDockerCompose("up", "-t", "0", "-d", "gitea")
	tr.dockerComposeLogToStd(t)

	// Wait for gitea to complete starting
	tr.mustRun(tr.wait())

	tr.mustRunDockerCompose("exec", "-T", "-u", "git", "gitea", "gitea", "migrate")

	tr.mustRunDockerCompose("exec", "-T", "-u", "git", "gitea", "gitea", "admin", "user", "create", "--admin",
		"--username", os.Getenv("PLAYWITHGODEV_ROOT_USER"),
		"--password", os.Getenv("PLAYWITHGODEV_ROOT_PASSWORD"),
		"--email", "blah@blah.com",
	)

	tokenJSON, _ := tr.mustRun(tr.self("newcontributor", "-email", "contributor@blah.com", "-fullname", "A Contributor", "-username", "testcontributor"))
	var token gitea.AccessToken
	err := json.Unmarshal(tokenJSON, &token)
	check(err, "failed to decode token from %q: %v", tokenJSON, err)

	// Stop the docker-compose setup
	tr.mustRunDockerCompose("down")

	// Start everything back up again with a modified environment
	// including the contributor details
	upContrib := tr.dockerComposeCmd("up", "-t", "0", "-d")
	upContrib.Env = append(os.Environ(),
		"PLAYWITHGODEV_CONTRIBUTOR_USER=newcontributor",
		"PLAYWITHGODEV_CONTRIBUTOR_PASSWORD="+token.Token,
	)
	tr.mustRun(upContrib)
	tr.dockerComposeLogToStd(t)

	// Run self again to create a new user, and then reap the user
	tr.mustRun(tr.prestep())
	tr.mustRun(tr.self("reap", "-age", "0s"))
}

func runTestStep(part string) int {
	var err error
	switch part {
	case "prestep":
		err = prestepErr()
	case "wait":
		err = waitErr()
	default:
		fmt.Fprintf(os.Stderr, "unknown step %q", part)
	}
	if err == nil {
		return 0
	}
	switch err := err.(type) {
	case usageErr:
		if err.err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, err.err)
		}
		fmt.Fprint(os.Stderr, err.u.usage())
		return 2
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func waitErr() (err error) {
	// Creating a gitea client attempts to connect to the version
	// endpoint. Do this until we succeed, or until some time has
	// passed
	strategy := retry.LimitTime(30*time.Second,
		retry.Exponential{
			Initial: 100 * time.Millisecond, // be slightly less aggressive with Vim
			Factor:  1.5,
		},
	)
	for a := retry.Start(strategy, nil); a.Next(); {
		_, err := gitea.NewClient("http://gitea:3000")
		if err == nil {
			break
		}
	}
	return nil
}

func prestepErr() (err error) {
	defer handleKnown(&err)
	args := strings.NewReader(`{"Repos": [{"Var": "REPO1", "Pattern": "user"},{"Var": "REPO2", "Pattern": "user*", "Private": true}]}`)
	newuserURL := "http://cmd_gitea:8080/newuser"
	resp, err := http.Post(newuserURL, "application/json", args)
	check(err, "failed to post to %v: %v", newuserURL, err)
	defer resp.Body.Close()
	newuser, err := io.ReadAll(resp.Body)
	check(err, "failed to read newuser response: %v", err)
	if resp.StatusCode/100 != 2 {
		raise("newuser request not successful with code %v: %s", resp.StatusCode, newuser)
	}

	dec := json.NewDecoder(bytes.NewBuffer(newuser))
	var env preguide.PrestepOut
	if err := dec.Decode(&env); err != nil {
		raise("failed to decode env information: %v. Input was: %s", err, newuser)
	}
	found := make(map[string]*string)
	for _, v := range env.Vars {
		val := v[strings.Index(v, "=")+1:]
		found[v[:strings.Index(v, "=")]] = &val
	}
	fail := false
	for k, v := range found {
		if v == nil {
			fail = true
			fmt.Fprintf(os.Stderr, "failed to find env var %v in %v", k, pretty.Sprint(env))
		}
	}
	if fail {
		raise("not all environment variables found")
	}
	// Verify that the patterns for both repo were respected
	repo1 := fmt.Sprintf("random.com/%v/user", *found["GITEA_USERNAME"])
	if *found["REPO1"] != repo1 {
		raise("expected REPO1 to be %q; got %q", repo1, *found["REPO1"])
	}
	if *found["REPO2"] == repo1 || !strings.HasPrefix(*found["REPO2"], repo1) {
		raise("expected REPO2 to have prefix %q; got %q", repo1, *found["REPO2"])
	}
	// TODO: reinstate some sort of test here: github.com/play-with-go/gitea/issues/69
	return nil
}

func (tr *testRunner) self(args ...string) *exec.Cmd {
	return tr.selfImpl("self", args...)
}

func (tr *testRunner) prestep(args ...string) *exec.Cmd {
	return tr.selfImpl("prestep", args...)
}

func (tr *testRunner) wait() *exec.Cmd {
	return tr.selfImpl("wait")
}

func (tr *testRunner) selfImpl(self string, args ...string) *exec.Cmd {
	cmd := exec.Command("docker", "run", "--rm",
		"-e", "GITEA_COMMAND="+self, "-e", "GITEA_ROOT_URL",
		"-e", "PLAYWITHGODEV_ROOT_USER", "-e", "PLAYWITHGODEV_ROOT_PASSWORD",
		"--network", tr.envComposeProjectName+"_gitea",
		"-v", fmt.Sprintf("%v:/giteaself", tr.selfPath), imageBase, "/giteaself")
	cmd.Args = append(cmd.Args, args...)
	return cmd
}

func (tr *testRunner) dockerComposeCmd(args ...string) *exec.Cmd {
	c := exec.Command("docker-compose", args...)
	c.Dir = tr.root
	return c
}

func (tr *testRunner) wrapEnv(cmd *exec.Cmd) *exec.Cmd {
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env,
		"COMPOSE_PROJECT_NAME="+tr.envComposeProjectName,
		"GITEA_ROOT_URL=http://random.com:3000",
	)
	return cmd
}

func (tr *testRunner) dockerComposeLogToStd(t *testing.T) {
	logs := tr.dockerComposeCmd("logs", "-f")
	logs.Stdout = os.Stdout
	logs.Stderr = os.Stderr
	logs.Env = append(os.Environ(),
		"COMPOSE_PROJECT_NAME="+tr.envComposeProjectName,
	)
	if err := logs.Start(); err != nil {
		t.Fatalf("failed to run logs process: %v", err)
	}
}

func (tr *testRunner) mustRunDockerCompose(args ...string) ([]byte, []byte) {
	return tr.mustRun(tr.dockerComposeCmd(args...))
}

func (tr *testRunner) mustRun(cmd *exec.Cmd) ([]byte, []byte) {
	tr.wrapEnv(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		tr.Fatalf("failed to run [%v]: %v\nstdout: %s\nstderr: %s", cmd, err, stdout.Bytes(), stderr.Bytes())
	}
	return stdout.Bytes(), stderr.Bytes()
}
