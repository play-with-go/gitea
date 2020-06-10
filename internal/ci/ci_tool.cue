package ci

import (
	"tool/exec"
	"tool/file"
	"encoding/yaml"
	"strings"
)

command: gengithub: {
	modRoot: exec.Run & {
		cmd:    "go list -m -f {{.Dir}}"
		stdout: string
	}
	write: file.Create & {
		filename: strings.TrimSpace(modRoot.stdout) + "/.github/workflows/test.yml"
		contents: """
			# Generated by ci_tool.cue; do not edit

			\(yaml.Marshal(test))
			"""
	}
}
