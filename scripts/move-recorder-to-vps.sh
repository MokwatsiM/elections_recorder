#!/usr/bin/env bash
# Moves the recorder from this machine to a VPS, keeping everything recorded so far.
# It adds a "lge" Docker project in /opt/lge and doesn't touch anything else there.
#
#   scripts/move-recorder-to-vps.sh root@your-server
#
# The local recorder stops only for the handover. If the handover fails, it restarts.
set -euo pipefail

REMOTE="${1:?usage: move-recorder-to-vps.sh user@host}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR=/opt/lge
LOCAL=(docker compose -f "$ROOT/deploy/docker-compose.yml")
REMOTE_COMPOSE="docker compose --env-file $DIR/deploy/.env -f $DIR/deploy/docker-compose.yml -f $DIR/deploy/docker-compose.vps.yml"
SSH=(ssh -o BatchMode=yes -o ConnectTimeout=15 "$REMOTE")

echo "== Checking the server"
"${SSH[@]}" 'docker compose version >/dev/null && df -h / | tail -1 && free -m | sed -n 2p'

echo "== Copying the project"
"${SSH[@]}" "mkdir -p $DIR/tiles"
rsync -az --delete --exclude bin/ "$ROOT/backend/" "$REMOTE:$DIR/backend/"
rsync -az --exclude .env "$ROOT/deploy/" "$REMOTE:$DIR/deploy/"
rsync -az --partial "$ROOT/tiles/sa.pmtiles" "$REMOTE:$DIR/tiles/sa.pmtiles"

echo "== Preparing secrets, the image and the database"
# Stays on the server, and is never overwritten by a later run.
"${SSH[@]}" "test -f $DIR/deploy/.env || (umask 077; printf 'POSTGRES_PASSWORD=%s\nIEC_USER_AGENT=%s\n' \"\$(openssl rand -hex 24)\" '${IEC_USER_AGENT:-}' > $DIR/deploy/.env)"
"${SSH[@]}" "$REMOTE_COMPOSE build backend && $REMOTE_COMPOSE up -d --wait postgres"

# Never replace a recording the server already has.
EXISTING=$("${SSH[@]}" "$REMOTE_COMPOSE exec -T postgres psql -U lge -d lge -Atc \"select to_regclass('raw_versions') is not null\"")
if [ "$EXISTING" = "t" ]; then
  echo "The server already has a recording, so I'm stopping rather than overwrite it." >&2
  exit 1
fi

echo "== Handing over"
restart_local() {
  echo "Handover failed, so I'm restarting the local recorder." >&2
  "${LOCAL[@]}" start backend || true
}
trap restart_local ERR

# Stopping first means no version is recorded after the dump and lost. Files only
# change about every 6 minutes, and the server's first sweep catches up on them.
"${LOCAL[@]}" stop backend
"${LOCAL[@]}" exec -T postgres pg_dump -U lge -d lge --format=custom \
  | "${SSH[@]}" "$REMOTE_COMPOSE exec -T postgres pg_restore -U lge -d lge --no-owner --exit-on-error"
"${SSH[@]}" "$REMOTE_COMPOSE up -d backend"
trap - ERR

echo "== Checking the server's recorder"
for _ in $(seq 1 60); do
  if "${SSH[@]}" 'curl -sf localhost:18080/readyz >/dev/null'; then
    "${SSH[@]}" 'curl -s localhost:18080/api/v1/status; echo'
    echo "Done. The local recorder is stopped, and its data is kept as a backup."
    exit 0
  fi
  sleep 2
done
echo "The server's recorder didn't become ready. Check it before relying on it:" >&2
echo "  ssh $REMOTE '$REMOTE_COMPOSE logs backend --tail 50'" >&2
exit 1
