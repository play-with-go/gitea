#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

source "$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/env_common.bash

$export GOPRIVATE github.com/play-with-go/*

modRoot="$(go list -m -f {{.Dir}})"

GOBIN=$modRoot/.bin go install github.com/myitcv/docker-compose

rootCA="$(mkcert -CAROOT)/rootCA.pem"
$export COMPOSE_PROJECT_NAME gitea
$export PATH "$modRoot/.bin:$PATH"
$export MKCERT_CAROOT_CERT "$rootCA"
