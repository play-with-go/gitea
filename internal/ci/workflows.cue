package ci

import "github.com/SchemaStore/schemastore/src/schemas/json"

workflows: [...{file: string, schema: json.#Workflow}]
workflows: [
	{file: "test.yml", schema: test},
]

_#latestGo:     "1.19.1"
_#ubuntuLatest: "ubuntu-22.04"

test: json.#Workflow & {
	name: "Test"
	env: {
		PLAYWITHGODEV_ROOT_USER:     "root"
		PLAYWITHGODEV_ROOT_PASSWORD: "asdffdsa"
	}
	on: {
		push: branches: ["main"]
		pull_request: branches: ["**"]
	}
	jobs: test: {
		strategy: {
			"fail-fast": false
			matrix: {
				os: [_#ubuntuLatest]
				go_version: [_#latestGo]
			}
		}
		"runs-on": "${{ matrix.os }}"
		steps: [
			{
				name: "Checkout code"
				uses: "actions/checkout@v2"
			},
			{
				name: "Install Go"
				uses: "actions/setup-go@v2"
				with: "go-version": "${{ matrix.go_version }}"
			},
			{
				name: "Env setup"
				run:  "./_scripts/env.sh github"
			},
			{
				name: "Verify"
				run:  "go mod verify"
			},
			{
				name: "docker-compose build"
				run:  "docker-compose build"
			},
			{
				name: "Generate"
				run:  "go generate ./..."
			},
			{
				name: "Test"
				run:  "go test ./..."
				env: CGO_ENABLED: "0"
			},
			{
				name: "staticcheck"
				run:  "go run honnef.co/go/tools/cmd/staticcheck ./..."
			},
			{
				name: "Tidy"
				run:  "go mod tidy"
			},
			{
				name: "Verify commit is clean"
				run:  #"test -z "$(git status --porcelain)" || (git status; git diff; false)"#
			},
		]
	}
}
