#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

# TODO: not ideal that this is a dependency of the main module
go install github.com/FiloSottile/mkcert
mkcert -install

setenv() {
	echo "::set-env name=$1::$(cat $2 | sed ':a;N;$!ba;s/%/%25/g' |  sed ':a;N;$!ba;s/\r/%0D/g' | sed ':a;N;$!ba;s/\n/%0A/g')"
}

td=$(mktemp -d)
trap "rm -rf $td" EXIT
mkcert -cert-file $td/cert.pem -key-file $td/key.pem gopher.live "*.gopher.live"

setenv PLAYWITHGODEV_CERT_FILE $td/cert.pem
setenv PLAYWITHGODEV_KEY_FILE $td/key.pem
