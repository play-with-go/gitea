package ci

import (
	"tool/file"
	"encoding/yaml"
)

command: gengithub: task: write: file.Create & {
	filename: "\(workflowsDir)/test.yml"
	contents: """
		# Generated by ci_tool.cue; do not edit

		\(yaml.Marshal(test))
		"""
}
