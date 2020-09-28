#!/usr/bin/env bash

set -eu -o pipefail
shopt -s inherit_errexit

eval "$($( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )/env.sh bash)"

args="$@"
if [ "$1" == "up" ] || [ "$1" == "down" ] || [ "$1" == "stop" ]
then
	first="$1 -t 0"
	shift
	args="$first $@"
fi

docker-compose $args
