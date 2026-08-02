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

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD must be set in $ENV_FILE}"
mkdir -p "$BACKUP_DIR"

# Dump everything except the ephemeral clips table.
docker compose -f "$COMPOSE_DIR/compose.yaml" exec -T postgres \
  pg_dump -U clipboard -d go_clipboard \
  --exclude-table-data=clips \
  | gzip -c >"$OUT"

echo "wrote $OUT"

if [[ -n "$BACKUP_REMOTE" ]]; then
  rsync -az "$OUT" "$BACKUP_REMOTE/"
  echo "copied to $BACKUP_REMOTE/"
fi

# Retain local copies for 7 days.
find "$BACKUP_DIR" -name 'go-clipboard-*.sql.gz' -mtime +7 -delete
