# Create a temporary docker-compose script and update PATH
td=$(mktemp -d)
# trap "rm -rf $td" EXIT
dc=$(which docker-compose)
cat <<EOD > $td/docker-compose
#!/usr/bin/env bash

$dc -f docker-compose.yml -f docker-compose-playwithgo.yml "\$@"
EOD
chmod +x $td/docker-compose
export PATH="$td:$PATH"

dockexec="go run mvdan.cc/dockexec -compose playwithgo -e PLAYWITHGODEV_ROOT_USER -e PLAYWITHGODEV_ROOT_PASSWORD -e PLAYWITHGODEV_GITHUB_USER -e PLAYWITHGODEV_GITHUB_PAT"
