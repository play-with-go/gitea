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
	"time"

	"github.com/kr/pretty"
	"github.com/play-with-go/preguide"
)

const (
	envComposeFile = "COMPOSE_FILE"
)

type testRunner struct {
	*testing.T
	root                  string
	envComposeFile        string
	envGoModCache         string
	envComposeProjectName string
}

func newTestRunner(t *testing.T) *testRunner {
	tr := &testRunner{
		T:                     t,
		envComposeProjectName: fmt.Sprintf("test%v", time.Now().UnixNano()),
	}
	listOut, _ := tr.mustRunCmd(exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/play-with-go/gitea"))
	root := strings.TrimSpace(string(listOut))
	composeFiles := []string{
		os.Getenv(envComposeFile),
		filepath.Join(root, "docker-compose.yml"),
		filepath.Join(root, "docker-compose-playwithgo.yml"),
	}
	var goenv struct {
		GOPATH string
	}
	envOut, _ := tr.mustRunCmd(exec.Command("go", "env", "-json"))
	if err := json.Unmarshal(envOut, &goenv); err != nil {
		t.Fatalf("failed to unmarshal go env output (%q): %v", envOut, err)
	}
	gopath0 := strings.Split(goenv.GOPATH, string(os.PathListSeparator))[0]

	tr.root = root
	tr.envComposeFile = strings.Join(composeFiles, string(os.PathListSeparator))
	tr.envGoModCache = filepath.Join(gopath0, "pkg", "mod")
	return tr
}

var mainRef bool

func TestNewUser(t *testing.T) {
	if mainRef {
		main()
	}
	tr := newTestRunner(t)
	// Start the docker-compose instance in the background
	tr.mustRunDockerCompose("up", "-t", "0", "-d")

	// Ensure it's torn down at the end of the test
	t.Cleanup(func() {
		tr.mustRunDockerCompose("down", "-t", "0", "-v")
	})

	tr.mustRunCmd(exec.Command("go", "run", "github.com/play-with-go/gitea/cmd/gitea", "setup"))
	newUser := exec.Command("go", "run", "github.com/play-with-go/gitea/cmd/gitea", "newuser")
	newUser.Stdin = strings.NewReader(`{"Repos": [{"Var": "REPO1", "Pattern": "user*"}]}`)
	newUserOut, _ := tr.mustRunCmd(newUser)

	dec := json.NewDecoder(bytes.NewBuffer(newUserOut))
	var versionDetails struct {
		Path string
	}
	if err := dec.Decode(&versionDetails); err != nil {
		t.Fatalf("failed to decode version details: %v. Input was: %s", err, newUserOut)
	}
	wantPath := "github.com/play-with-go/gitea/cmd/gitea"
	if gotPath := versionDetails.Path; wantPath != gotPath {
		t.Fatalf("wanted version path %v; got %v", wantPath, gotPath)
	}
	var env preguide.PrestepOut
	if err := dec.Decode(&env); err != nil {
		t.Fatalf("failed to decode env information: %v. Input was: %s", err, newUserOut)
	}
	found := map[string]bool{
		"GITEA_USERNAME": false,
		"GITEA_PASSWORD": false,
		"REPO1":          false,
	}
	for _, v := range env.Vars {
		found[v[:strings.Index(v, "=")]] = true
	}
	for k, v := range found {
		if !v {
			t.Errorf("failed to find env var %v in %v", k, pretty.Sprint(env))
		}
	}
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
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env,
		"COMPOSE_PROJECT_NAME="+tr.envComposeProjectName,
	)
	err := cmd.Run()
	if err != nil {
		tr.Fatalf("failed to run [%v]: %v\n%s", cmd, err, stderr.Bytes())
	}
	return stdout.Bytes(), stderr.Bytes()
}
