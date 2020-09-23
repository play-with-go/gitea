package ci

import "github.com/SchemaStore/schemastore/src/schemas/json"

workflows: [...{file: string, schema: json.#Workflow}]
workflows: [
	{file: "test.yml", schema: test},
]

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
		push: branches: ["main"]
		pull_request: branches: ["**"]
	}
	jobs: test: {
		strategy: {
			"fail-fast": false
			matrix: {
				os: ["ubuntu-latest"]
				go_version: ["1.15"]
			}
		}
		"runs-on": "${{ matrix.os }}"
		steps: [{
			name: "Setup netrc"
			run:  #"echo -e "machine github.com\nlogin $PLAYWITHGODEV_GITHUB_USER\npassword $PLAYWITHGODEV_GITHUB_PAT\n\nmachine api.github.com\nlogin $PLAYWITHGODEV_GITHUB_USER\npassword $PLAYWITHGODEV_GITHUB_PAT\n" > ~/.netrc"#
		}, {
			name: "Checkout code"
			uses: "actions/checkout@v2"
		}, {
			name: "Install Go"
			uses: "actions/setup-go@v2"
			with: "go-version": "${{ matrix.go_version }}"
		}, {
			name: "Setup mkcert"
			run:  "./_scripts/setupmkcert.sh"
		}, {
			name: "Env setup"
			run:  "./_scripts/env.sh github"
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
			name: "Tidy"
			run:  "go mod tidy"
		}, {
			name: "Verify commit is clean"
			run:  #"test -z "$(git status --porcelain)" || (git status; git diff; false)"#
		}]
	}
}
