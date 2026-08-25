#!/usr/bin/env bash
# Restore from a pg_dump produced by backup.sh.
# Live clips are not in the dump and will be empty after restore — that is expected.
set -euo pipefail

DUMP="${1:?usage: restore.sh <backup.sql.gz>}"
COMPOSE_DIR="${COMPOSE_DIR:-/opt/go-clipboard}"
ENV_FILE="${ENV_FILE:-$COMPOSE_DIR/.env}"
PROJECT="${COMPOSE_PROJECT_NAME:-go-clipboard}"
NETWORK="${PROJECT}_internal"
PG_CLIENT_IMAGE="${PG_CLIENT_IMAGE:-postgres:18-alpine}"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

echo "Restoring $DUMP (clips data is not preserved by design)."
if [[ -n "${DATABASE_URL:-}" ]]; then
  # External database — see the matching branch in backup.sh.
  gunzip -c "$DUMP" | docker run --rm -i --network "$NETWORK" -e DATABASE_URL \
    "$PG_CLIENT_IMAGE" sh -c 'exec psql "$DATABASE_URL"'
else
  : "${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD (bundled database) or DATABASE_URL (external database) in $ENV_FILE}"
  gunzip -c "$DUMP" | docker compose -f "$COMPOSE_DIR/compose.yaml" exec -T postgres \
    psql -U clipboard -d go_clipboard
fi
echo "restore complete"
