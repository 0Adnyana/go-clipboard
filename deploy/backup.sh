#!/usr/bin/env bash
# Nightly pg_dump of non-ephemeral data. Ephemeral `clips` are excluded — a
# restore that loses every live clip is an accepted outcome.
set -euo pipefail

COMPOSE_DIR="${COMPOSE_DIR:-/opt/go-clipboard}"
ENV_FILE="${ENV_FILE:-$COMPOSE_DIR/.env}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/go-clipboard}"
# Off-box destination (rsync/scp). Empty skips the copy step.
BACKUP_REMOTE="${BACKUP_REMOTE:-}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$BACKUP_DIR/go-clipboard-$STAMP.sql.gz"
PROJECT="${COMPOSE_PROJECT_NAME:-go-clipboard}"
NETWORK="${PROJECT}_internal"
# Client tools for an external database. Must be at least the server's major
# version; override when the external server is newer than this image.
PG_CLIENT_IMAGE="${PG_CLIENT_IMAGE:-postgres:18-alpine}"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

mkdir -p "$BACKUP_DIR"

# Dump everything except the ephemeral clips table.
if [[ -n "${DATABASE_URL:-}" ]]; then
  # External database: run client tools in a throwaway container. The URL is
  # passed as an env var, not argv, to keep the password out of the host's
  # process list.
  docker run --rm --network "$NETWORK" -e DATABASE_URL "$PG_CLIENT_IMAGE" \
    sh -c 'exec pg_dump "$DATABASE_URL" --exclude-table-data=clips' \
    | gzip -c >"$OUT"
else
  : "${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD (bundled database) or DATABASE_URL (external database) in $ENV_FILE}"
  docker compose -f "$COMPOSE_DIR/compose.yaml" exec -T postgres \
    pg_dump -U clipboard -d go_clipboard \
    --exclude-table-data=clips \
    | gzip -c >"$OUT"
fi

echo "wrote $OUT"

if [[ -n "$BACKUP_REMOTE" ]]; then
  rsync -az "$OUT" "$BACKUP_REMOTE/"
  echo "copied to $BACKUP_REMOTE/"
fi

# Retain local copies for 7 days.
find "$BACKUP_DIR" -name 'go-clipboard-*.sql.gz' -mtime +7 -delete
