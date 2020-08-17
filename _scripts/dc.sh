#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

eval "$($( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )/env.sh bash)"

export GOMODCACHE="$(go env GOPATH | cut -f 1 -d :)"/pkg/mod

docker-compose "$@"
