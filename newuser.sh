#!/usr/bin/env bash

set -eu

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

go run -exec "go run mvdan.cc/dockexec buildpack-deps@sha256:ec0e9539673254d0cb1db0de347905cdb5d5091df95330f650be071a7e939420 --network=gitea_gitea --rm -e PLAYWITHGODEV_ROOT_USER -e PLAYWITHGODEV_ROOT_PASSWORD -e PLAYWITHGODEV_GITHUB_USER -e PLAYWITHGODEV_GITHUB_PAT" . newuser "$@" raw > $tf

chmod 700 $tf

if [ "$command" == "run" ]
then
	docker run --rm -e USER_UID=$(id -u) -e USER_GID=$(id -g) --network gitea_gitea -v $PWD:/scripts playwithgo/go1.14.3@sha256:6289a34af0112146e551790d9f2a36622e083600752a02d36f1ad1f9e1389382 /scripts/$(basename $tf)
else
	cat $tf
fi
