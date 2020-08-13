package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kr/pretty"
	"github.com/play-with-go/preguide"
)

const (
	envComposeFile = "COMPOSE_FILE"
)

type testRunner struct {
	*testing.T
	root           string
	envComposeFile string
	envGoModCache  string
}

func newTestRunner(t *testing.T) *testRunner {
	var listOut, listErr bytes.Buffer
	list := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/play-with-go/gitea")
	list.Stdout = &listOut
	list.Stderr = &listErr
	if err := list.Run(); err != nil {
		t.Fatalf("failed to get root of module: %v\n%s", err, listErr.Bytes())
	}
	root := strings.TrimSpace(listOut.String())
	composeFiles := []string{
		os.Getenv(envComposeFile),
		filepath.Join(root, "docker-compose.yml"),
		filepath.Join(root, "docker-compose-playwithgo.yml"),
	}
	var goenv struct {
		GOPATH string
	}
	var envOut, envErr bytes.Buffer
	env := exec.Command("go", "env", "-json")
	env.Stdout = &envOut
	env.Stderr = &envErr
	if err := env.Run(); err != nil {
		t.Fatalf("failed to get go env: %v\n%s", err, envErr.Bytes())
	}
	if err := json.Unmarshal(envOut.Bytes(), &goenv); err != nil {
		t.Fatalf("failed to unmarshal go env output (%q): %v", envOut.Bytes(), err)
	}
	gopath0 := strings.Split(goenv.GOPATH, string(os.PathListSeparator))[0]

	return &testRunner{
		T:              t,
		root:           root,
		envComposeFile: strings.Join(composeFiles, string(os.PathListSeparator)),
		envGoModCache:  filepath.Join(gopath0, "pkg", "mod"),
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

	tr.mustRunCmd(exec.Command("go", "run", "github.com/play-with-go/gitea/cmd/gitea", "setup"))
	newUser := exec.Command("go", "run", "github.com/play-with-go/gitea/cmd/gitea", "newuser")
	newUser.Stdin = strings.NewReader(`{"Repos": [{"Var": "REPO1", "Pattern": "user*"}]}`)
	outJSON, _ := tr.mustRunCmd(newUser)

	var env preguide.PrestepOut

	// Verify we got some valid JSON back
	if err := json.Unmarshal(outJSON, &env); err != nil {
		t.Fatalf("failed to decode preguide.PrestepOut from %q: %v", outJSON, err)
	}
	fmt.Printf("Env vars: %v\n", pretty.Sprint(env))
}

func (tr *testRunner) mustRunDockerCompose(args ...string) ([]byte, []byte) {
	c := exec.Command("docker-compose", args...)
	c.Dir = tr.root
	c.Env = append(os.Environ(),
		"COMPOSE_FILE="+tr.envComposeFile,
		"GOMODCACHE="+tr.envGoModCache,
	)
	return tr.mustRunCmd(c)
}

func (tr *testRunner) mustRunCmd(cmd *exec.Cmd) ([]byte, []byte) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		tr.Fatalf("failed to run [%v]: %v\n%s", cmd, err, stderr.Bytes())
	}
	return stdout.Bytes(), stderr.Bytes()
}
