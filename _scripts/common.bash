root="$( command cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )/../"
export COMPOSE_FILE="${COMPOSE_FILE:-}:$root/docker-compose.yml:$root/docker-compose-playwithgo.yml"
dockexec="go run mvdan.cc/dockexec -compose playwithgo -e PLAYWITHGODEV_ROOT_USER -e PLAYWITHGODEV_ROOT_PASSWORD -e PLAYWITHGODEV_GITHUB_USER -e PLAYWITHGODEV_GITHUB_PAT"
