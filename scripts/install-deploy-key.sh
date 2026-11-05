#!/usr/bin/env bash
# Sets up CI deploys: installs lge-deploy on the server, creates a key limited to it
# and stores what the workflows need as GitHub secrets. Safe to run again, which also
# replaces the key. Needs gh logged in, and run from inside the repo.
#
#   scripts/install-deploy-key.sh root@your-server
set -euo pipefail

REMOTE="${1:?usage: install-deploy-key.sh user@host}"
HOST="${REMOTE#*@}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner | tr '[:upper:]' '[:lower:]')
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

ssh-keygen -q -t ed25519 -N '' -C "lge-github-actions" -f "$TMP/key"
LINE="restrict,command=\"/usr/local/bin/lge-deploy\" $(cat "$TMP/key.pub")"

scp -q "$ROOT/deploy/lge-deploy.sh" "$REMOTE:/usr/local/bin/lge-deploy"
ssh "$REMOTE" "chmod 755 /usr/local/bin/lge-deploy
  echo 'ghcr.io/$REPO' > /opt/lge/deploy/image-prefix
  sed -i '/lge-github-actions/d' ~/.ssh/authorized_keys
  echo '$LINE' >> ~/.ssh/authorized_keys"

gh secret set VPS_HOST --body "$HOST"
gh secret set VPS_KNOWN_HOSTS --body "$(ssh-keyscan -q -t ed25519 "$HOST")"
gh secret set VPS_SSH_KEY < "$TMP/key"
echo "Done. CI can now deploy to $HOST."
