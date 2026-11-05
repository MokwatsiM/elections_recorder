#!/usr/bin/env bash
# Copies a full database backup from the VPS to backups/ on this machine.
#   scripts/backup-from-vps.sh root@your-server
set -euo pipefail

REMOTE="${1:?usage: backup-from-vps.sh user@host}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$ROOT/backups"
OUT="$ROOT/backups/lge-$(date +%Y%m%d-%H%M).dump"

ssh -o BatchMode=yes "$REMOTE" 'docker exec lge-postgres-1 pg_dump -U lge -d lge --format=custom' > "$OUT.partial"
# Confirms the copy is a complete, readable backup before keeping it.
ssh -o BatchMode=yes "$REMOTE" 'docker exec -i lge-postgres-1 pg_restore --list > /dev/null' < "$OUT.partial"
mv "$OUT.partial" "$OUT"
ls -lh "$OUT"
