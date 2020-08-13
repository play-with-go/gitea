#!/usr/bin/env bash

set -eu

source "$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/common.bash

# Run from the root
command cd "$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/..

if [ "$#" -eq 0 ]
then
	echo "need a command, either run or echo"
	exit 1
fi

command="$1"
shift


if [ "$command" != "run" ] && [ "$command" != "echo" ]
then
	echo "unknown command; should be either run or echo"
	exit 1
fi

tf=$(mktemp tmpXXXXXX.sh)
trap "rm -f $tf" EXIT

go run -exec "$dockexec -T" github.com/play-with-go/gitea/cmd/gitea newuser "$@" raw > $tf

if [ "$command" == "run" ]
then
	docker-compose -f docker-compose.yml -f docker-compose-playwithgo.yml run --rm -e USER_UID=$(id -u) -e USER_GID=$(id -g) -v $PWD:/scripts playwithgo bash /scripts/$(basename $tf)
else
	cat $tf
fi
