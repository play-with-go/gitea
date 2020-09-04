#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

source "$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/env_common.bash

modRoot="$(go list -m -f {{.Dir}})"

GOBIN=$modRoot/.bin go install github.com/myitcv/docker-compose

rootCA="$(cat $(mkcert -CAROOT)/rootCA.pem)"
$export COMPOSE_PROJECT_NAME gitea
$export PATH "$modRoot/.bin:$PATH"
$export PLAYWITHGO_ROOTCA "$rootCA"
