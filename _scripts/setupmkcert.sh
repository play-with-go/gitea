#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

command cd "$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )/mkcert"
go install github.com/FiloSottile/mkcert
mkcert -install

td=$(mktemp -d)
trap "rm -rf $td" EXIT
mkcert -cert-file $td/cert.pem -key-file $td/key.pem play-with-go.dev "*.play-with-go.dev"
go run . $td

