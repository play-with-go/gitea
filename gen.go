package main

//go:generate go run cuelang.org/go/cmd/cue cmd vendorgithubschema github.com/play-with-go/gitea
//go:generate go run cuelang.org/go/cmd/cue cmd -t workflowsDir=./.github/workflows gengithub ./internal/ci
