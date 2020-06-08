#!/usr/bin/env bash

set -eu

command cd "$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

export COMPOSE_PROJECT_NAME=gitea

docker-compose stop
for i in playwithgodev_gitea playwithgodev_nginx
do
	docker inspect $i > /dev/null 2>&1 && docker rm $i
done

docker network rm gitea_gitea || true
docker volume rm gitea_gitea || true

# Start for init and then stop
docker-compose up -d
docker-compose stop

# Setup database
docker-compose run --rm -u git gitea gitea migrate

# Create admin user
docker-compose run --rm -u git gitea gitea admin create-user --username $PLAYWITHGODEV_ROOT_USER --password $PLAYWITHGODEV_ROOT_PASSWORD --admin --email blah@blah.com

# Start for good
docker-compose up -d

dockexec="go run mvdan.cc/dockexec buildpack-deps@sha256:ec0e9539673254d0cb1db0de347905cdb5d5091df95330f650be071a7e939420 --network=gitea_gitea --rm -e PLAYWITHGODEV_ROOT_USER -e PLAYWITHGODEV_ROOT_PASSWORD -e PLAYWITHGODEV_GITHUB_USER -e PLAYWITHGODEV_GITHUB_PAT"

# Wait for API to serve
go run -exec "$dockexec" . wait

# Create userguides org
go run -exec "$dockexec" . pre
