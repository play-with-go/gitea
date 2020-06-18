#!/usr/bin/env bash

set -eu

source "$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"/common.bash

docker-compose stop gitea

# Setup database
docker-compose run --rm -u git gitea gitea migrate

# Create admin user
docker-compose run --rm -u git gitea gitea admin create-user --username $PLAYWITHGODEV_ROOT_USER --password $PLAYWITHGODEV_ROOT_PASSWORD --admin --email blah@blah.com

# Start for good
docker-compose up -d gitea

# Wait for API to serve
go run -exec "$dockexec" github.com/play-with-go/gitea wait

# Create userguides org
go run -exec "$dockexec" github.com/play-with-go/gitea pre
