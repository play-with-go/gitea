package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	envComposeFile = "COMPOSE_FILE"
)

type testRunner struct {
	*testing.T
}

func TestNewUser(t *testing.T) {
	tr := testRunner{t}
	// In case the docker-compose instance is already running... blow it away
	// so that the test starts from fresh
	tr.mustRunDockerCompose("down", "-t", "0", "-v")

	// Start the docker-compose instance in the background
	tr.mustRunDockerCompose("up", "-t", "0", "-d")

	// Ensure it's torn down at the end of the test
	t.Cleanup(func() {
		tr.mustRunDockerCompose("down", "-t", "0", "-v")
	})

	tr.mustRun("go", "run", "github.com/play-with-go/gitea", "setup")
	tr.mustRun("./_scripts/newuser.sh", "run", "-test")
}

func (tr *testRunner) mustRun(cmd string, args ...string) string {
	return tr.mustRunCmd(exec.Command(cmd, args...))
}

func (tr *testRunner) mustRunDockerCompose(args ...string) string {
	cwd, err := os.Getwd()
	if err != nil {
		tr.Fatalf("failed to get working directory: %v", err)
	}
	composeFiles := []string{
		os.Getenv(envComposeFile),
		filepath.Join(cwd, "docker-compose.yml"),
		filepath.Join(cwd, "docker-compose-playwithgo.yml"),
	}
	c := exec.Command("docker-compose", args...)
	c.Env = append(os.Environ(),
		"COMPOSE_FILE="+strings.Join(composeFiles, string(os.PathListSeparator)),
	)
	return tr.mustRunCmd(c)
}

func (tr *testRunner) mustRunCmd(cmd *exec.Cmd) string {
	out, err := cmd.CombinedOutput()
	if err != nil {
		tr.Fatalf("failed to run [%v]: %v\n%s", cmd, err, out)
	}
	return string(out)
}
