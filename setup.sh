#!/usr/bin/env bash

set -eu

cd "${BASH_SOURCE%/*}"

docker-compose stop
for i in playwithgodev_gitea playwithgodev_nginx
do
	docker inspect $i > /dev/null 2>&1 && docker rm $i
done

rm -rf gitea

# gitea conf
mkdir -p gitea/gitea/conf
cp app.ini gitea/gitea/conf

docker-compose up --no-start

# Setup database
docker-compose run --rm -u git gitea gitea migrate

# Create admin user
docker-compose run --rm -u git gitea gitea admin create-user --username root --password asdffdsa --admin --email blah@blah.com

docker-compose up -d

# Wait for API to serve
go run -exec $PWD/dockexec.sh . wait

# Create userguides org
go run -exec $PWD/dockexec.sh . pre
