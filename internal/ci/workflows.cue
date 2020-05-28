package ci

import "github.com/SchemaStore/schemastore/src/schemas/json"

workflowsDir: *"./" | string @tag(workflowsDir)
scriptsDir:   *"./" | string @tag(scriptsDir)

test: json.#Workflow & {
	name: "Test"
	env: {
		PLAYWITHGODEV_ROOT_USER:     "root"
		PLAYWITHGODEV_ROOT_PASSWORD: "asdffdsa"
		PLAYWITHGODEV_GITHUB_USER:   "playwithgopher"
		PLAYWITHGODEV_GITHUB_PAT:    "${{ secrets.PLAYWITHGODEV_GITHUB_PAT }}"
		PLAYWITHGODEV_CERT_FILE:     "${{ secrets.PLAYWITHGODEV_CERT_FILE }}"
		PLAYWITHGODEV_KEY_FILE:      "${{ secrets.PLAYWITHGODEV_KEY_FILE }}"
	}
	on: {
		pull_request: branches: [
			"**",
		]
		push: branches: [
			"master",
		]
	}
	jobs: test: {
		"runs-on": "ubuntu-latest"
		steps: [{
			name: "Checkout code"
			uses: "actions/checkout@v2"
		}, {
			name: "Install Go"
			uses: "actions/setup-go@v2"
			with: "go-version": "1.14.3"
		}, {
			name: "Verify"
			run:  "go mod verify"
		}, {
			name: "Generate"
			run:  "go generate ./..."
		}, {
			name: "Test"
			run:  "go test ./..."
		}, {
			name: "Run setup"
			run:  "./setup.sh"
		}, {
			name: "Create new user"
			run:  "./newuser.sh run -test"
		}, {
			name: "Tidy"
			run:  "go mod tidy"
		}, {
			name: "Verify commit is clean"
			run:  #"test -z "$(git status --porcelain)" || (git status; git diff; false)"#
		}]
	}
}
