#!/usr/bin/env bash

set -eu

go run -exec "go run mvdan.cc/dockexec buildpack-deps@sha256:ec0e9539673254d0cb1db0de347905cdb5d5091df95330f650be071a7e939420 --network=gitea_gitea --rm -e PLAYWITHGODEV_ROOT_USER -e PLAYWITHGODEV_ROOT_PASSWORD -e PLAYWITHGODEV_GITHUB_USER -e PLAYWITHGODEV_GITHUB_PAT" . newuser
