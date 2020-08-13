package main

import (
	"bytes"
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
	root string
}

func newTestRunner(t *testing.T) *testRunner {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/play-with-go/gitea")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to get root of module: %v\n%s", err, stderr.Bytes())
	}
	return &testRunner{
		T:    t,
		root: strings.TrimSpace(stdout.String()),
	}
}

func TestNewUser(t *testing.T) {
	tr := newTestRunner(t)
	// In case the docker-compose instance is already running... blow it away
	// so that the test starts from fresh
	tr.mustRunDockerCompose("down", "-t", "0", "-v")

	// Start the docker-compose instance in the background
	tr.mustRunDockerCompose("up", "-t", "0", "-d")

	// Ensure it's torn down at the end of the test
	t.Cleanup(func() {
		tr.mustRunDockerCompose("down", "-t", "0", "-v")
	})

	tr.mustRun("go", "run", "github.com/play-with-go/gitea/cmd/gitea", "setup")
	tr.mustRun("./_scripts/newuser.sh", "run", "-test")
}

func (tr *testRunner) mustRun(cmd string, args ...string) string {
	c := exec.Command(cmd, args...)
	c.Dir = tr.root
	return tr.mustRunCmd(c)
}

func (tr *testRunner) mustRunDockerCompose(args ...string) string {
	composeFiles := []string{
		os.Getenv(envComposeFile),
		filepath.Join(tr.root, "docker-compose.yml"),
		filepath.Join(tr.root, "docker-compose-playwithgo.yml"),
	}
	c := exec.Command("docker-compose", args...)
	c.Dir = tr.root
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
