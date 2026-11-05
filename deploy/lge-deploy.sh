#!/usr/bin/env bash
# Runs on the server as the forced command of the CI deploy key, so that key can do
# only this. Install it with scripts/install-deploy-key.sh.
#
#   config                    reads a tar of the compose files on stdin
#   deploy <service> <sha>    reads "<registry user>\n<registry token>" on stdin
#
# A deploy that fails its health check goes back to the previous image.
set -euo pipefail

DIR=/opt/lge/deploy
IMAGES="$DIR/images.env"
PREFIX=$(cat "$DIR/image-prefix") # ghcr.io/<owner>/<repo>
COMPOSE=(docker compose --env-file "$DIR/.env" --env-file "$IMAGES"
  -f "$DIR/docker-compose.yml" -f "$DIR/docker-compose.vps.yml" --profile private --profile tunnel)

exec 9>/run/lge-deploy.lock
flock 9
touch "$IMAGES"

healthy() {
  local url=$1
  for _ in $(seq 1 60); do
    curl -sf -o /dev/null --max-time 5 "$url" && return 0
    sleep 2
  done
  return 1
}

set_image() { # <VAR> <image, or empty for the default>
  sed -i "/^$1=/d" "$IMAGES"
  if [ -n "$2" ]; then echo "$1=$2" >> "$IMAGES"; fi
}

read -r action service sha extra <<< "${SSH_ORIGINAL_COMMAND:-}"

case "$action" in
  config)
    tar -xzf - --no-same-owner -C "$DIR" docker-compose.yml docker-compose.vps.yml
    echo "Compose files updated"
    ;;
  deploy)
    [ -z "${extra:-}" ] && [[ "$sha" =~ ^[0-9a-f]{40}$ ]] || { echo "Bad arguments" >&2; exit 2; }
    case "$service" in
      frontend) var=FRONTEND_IMAGE; url=http://localhost:13000/api/v1/status ;;
      backend) var=BACKEND_IMAGE; url=http://localhost:18080/readyz ;;
      *) echo "Unknown service" >&2; exit 2 ;;
    esac
    image="$PREFIX-$service:$sha"

    # The token lives only in this throwaway config, and only for the pull.
    read -r user
    read -r token
    conf=$(mktemp -d)
    trap 'rm -rf "$conf"' EXIT
    printf '%s' "$token" | docker --config "$conf" login ghcr.io -u "$user" --password-stdin >/dev/null
    docker --config "$conf" pull -q "$image"
    rm -rf "$conf"

    previous=$(sed -n "s/^$var=//p" "$IMAGES")
    set_image "$var" "$image"
    "${COMPOSE[@]}" up -d --no-build --no-deps --pull never "$service"

    if ! healthy "$url"; then
      echo "$service failed its health check, going back to ${previous:-the server-built image}" >&2
      set_image "$var" "$previous"
      "${COMPOSE[@]}" up -d --no-build --no-deps --pull never "$service"
      healthy "$url" || echo "The previous image isn't healthy either" >&2
      exit 1
    fi

    # Keep the last three images for rollbacks.
    docker images "$PREFIX-$service" --format '{{.Repository}}:{{.Tag}}' | tail -n +4 | xargs -r docker rmi >/dev/null 2>&1 || true
    echo "Deployed $image"
    ;;
  *)
    echo "Usage: config | deploy <frontend|backend> <sha>" >&2
    exit 2
    ;;
esac
