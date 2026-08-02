#!/usr/bin/env bash
# Restore from a pg_dump produced by backup.sh.
# Live clips are not in the dump and will be empty after restore — that is expected.
set -euo pipefail

DUMP="${1:?usage: restore.sh <backup.sql.gz>}"
COMPOSE_DIR="${COMPOSE_DIR:-/opt/go-clipboard}"
ENV_FILE="${ENV_FILE:-$COMPOSE_DIR/.env}"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set in $ENV_FILE}"

echo "Restoring $DUMP (clips data is not preserved by design)."
gunzip -c "$DUMP" | docker compose -f "$COMPOSE_DIR/compose.yaml" exec -T postgres \
  psql -U clipboard -d go_clipboard
echo "restore complete"
