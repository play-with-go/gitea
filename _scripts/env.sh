#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

source "$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/env_common.bash

modRoot="$(go list -m -f {{.Dir}})"

$export COMPOSE_PROJECT_NAME gitea
$export PATH "$modRoot/.bin:$PATH"
$export COMPOSE_DOCKER_CLI_BUILD 1
$export DOCKER_BUILDKIT 1
$export CGO_ENABLED=0
