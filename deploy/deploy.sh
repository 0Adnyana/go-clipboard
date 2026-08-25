#!/usr/bin/env bash
# Deploy a digest-pinned or SHA-tagged image on the production host.
# Run from the directory that contains compose.yaml (default /opt/go-clipboard).
set -euo pipefail

IMAGE="${1:?usage: deploy.sh <image-ref> [previous-image-ref]}"
PREVIOUS_IMAGE="${2:-}"

reject_moving_tag() {
  local ref="$1"
  if [[ "$ref" == *:latest || "$ref" == */latest || "$ref" == latest ]]; then
    echo "refusing to deploy moving tag: $ref (use a full commit SHA tag or digest)" >&2
    exit 1
  fi
  if [[ "$ref" == *@sha256:* ]]; then
    return 0
  fi
  local tag="${ref##*:}"
  if [[ ! "$tag" =~ ^[0-9a-f]{40}$ ]]; then
    echo "refusing deploy: image must be a digest (@sha256:…) or full 40-char SHA tag, got: $ref" >&2
    exit 1
  fi
}

reject_moving_tag "$IMAGE"

image_identity() {
  local ref="$1"
  if [[ "$ref" == *@sha256:* ]]; then
    printf '%s\n' "${ref##*@}"
    return
  fi
  printf '%s\n' "${ref##*:}"
}

# After docker pull, require RepoDigests to contain the expected digest.
# SHA-tag refs skip this unless EXPECTED_DIGEST is set (CI deploys by digest).
assert_repo_digest() {
  local ref="$1"
  local expected=""
  if [[ "$ref" == *@sha256:* ]]; then
    expected="${ref##*@}"
  elif [[ -n "${EXPECTED_DIGEST:-}" ]]; then
    expected="$EXPECTED_DIGEST"
  else
    return 0
  fi
  if [[ ! "$expected" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "invalid expected digest for $ref: $expected" >&2
    exit 1
  fi
  local digests line suffix
  digests="$(docker inspect --format='{{range .RepoDigests}}{{println .}}{{end}}' "$ref" 2>/dev/null || true)"
  if [[ -z "$digests" ]]; then
    echo "pulled image $ref has no RepoDigests; cannot verify $expected" >&2
    exit 1
  fi
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    suffix="${line##*@}"
    if [[ "$suffix" == "$expected" ]]; then
      echo "verified RepoDigest $expected"
      return 0
    fi
  done <<<"$digests"
  echo "pulled image $ref RepoDigests do not contain $expected:" >&2
  echo "$digests" >&2
  exit 1
}

COMPOSE_DIR="${COMPOSE_DIR:-/opt/go-clipboard}"
ENV_FILE="${ENV_FILE:-$COMPOSE_DIR/.env}"
HEALTH_TIMEOUT_SEC="${HEALTH_TIMEOUT_SEC:-90}"
LOCK_FILE="${LOCK_FILE:-/var/lock/go-clipboard-deploy.lock}"
STATE_FILE="${STATE_FILE:-$COMPOSE_DIR/deployed-sha}"
PROJECT="${COMPOSE_PROJECT_NAME:-go-clipboard}"
NETWORK="${PROJECT}_internal"
CANDIDATE_NAME=go-clipboard-candidate

cd "$COMPOSE_DIR"

# Load host secrets (POSTGRES_PASSWORD, PUBLIC_BASE_URL, …).
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${PUBLIC_BASE_URL:?PUBLIC_BASE_URL must be set in $ENV_FILE}"
: "${TRUSTED_PROXY:?TRUSTED_PROXY must be set in $ENV_FILE}"

# A DATABASE_URL in the env file selects an external database and turns off the
# bundled postgres service; otherwise target the bundled one. Keep the fallback
# identical to the DATABASE_URL default in compose.yaml.
if [[ -n "${DATABASE_URL:-}" ]]; then
  BUNDLED_DB=0
  export COMPOSE_PROFILES=""
else
  BUNDLED_DB=1
  export COMPOSE_PROFILES="bundled-db"
  : "${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD (bundled database) or DATABASE_URL (external database) in $ENV_FILE}"
  DATABASE_URL="postgres://clipboard:${POSTGRES_PASSWORD}@postgres:5432/go_clipboard?sslmode=disable"
fi

# Host header must match PUBLIC_BASE_URL (enforced by the server).
PUBLIC_HOST="${PUBLIC_BASE_URL#https://}"
PUBLIC_HOST="${PUBLIC_HOST%%/*}"
PUBLIC_HOST="${PUBLIC_HOST%%:*}"

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  echo "another deploy holds $LOCK_FILE; aborting" >&2
  exit 1
fi

# Refuse to clobber a live deployment that is not the expected predecessor.
NEW_ID="$(image_identity "$IMAGE")"
if [[ -n "${EXPECTED_PREVIOUS_SHA:-}" && -f "$STATE_FILE" ]]; then
  LIVE="$(cat "$STATE_FILE")"
  if [[ "$LIVE" != "$EXPECTED_PREVIOUS_SHA" && "$LIVE" != "$NEW_ID" ]]; then
    echo "live deployment is $LIVE; refusing to replace it with $NEW_ID (expected previous $EXPECTED_PREVIOUS_SHA)" >&2
    exit 1
  fi
fi

# DATABASE_URL is exported so compose interpolation resolves to the same target
# the migrate and candidate containers below use.
export IMAGE ENV_FILE DATABASE_URL

if [[ "$BUNDLED_DB" -eq 1 ]]; then
  echo "ensuring bundled postgres is up"
  docker compose -f "$COMPOSE_DIR/compose.yaml" up -d postgres
else
  echo "using external database from DATABASE_URL in $ENV_FILE"
fi

wait_healthy() {
  local target="$1"
  local i
  for i in $(seq 1 "$HEALTH_TIMEOUT_SEC"); do
    if docker run --rm --network "$NETWORK" curlimages/curl:8.5.0 \
      -fsS --max-time 3 -H "Host: ${PUBLIC_HOST}" \
      "http://${target}:8080/api/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

echo "pulling $IMAGE"
docker pull "$IMAGE"
assert_repo_digest "$IMAGE"

echo "applying migrations via image (never on container start)"
docker run --rm \
  --network "$NETWORK" \
  -e "DATABASE_URL=$DATABASE_URL" \
  -e "MIGRATIONS_DIR=/app/db/migrations" \
  "$IMAGE" migrate up

# Release gate: previous image must remain healthy against the migrated schema.
if [[ -n "$PREVIOUS_IMAGE" && "$PREVIOUS_IMAGE" != "$IMAGE" ]]; then
  reject_moving_tag "$PREVIOUS_IMAGE"
  echo "rollback-compatibility check: $PREVIOUS_IMAGE"
  docker pull "$PREVIOUS_IMAGE"
  assert_repo_digest "$PREVIOUS_IMAGE"
  docker rm -f go-clipboard-prevcheck >/dev/null 2>&1 || true
  docker run -d --name go-clipboard-prevcheck --rm \
    --network "$NETWORK" \
    -e "DATABASE_URL=$DATABASE_URL" \
    -e "MIGRATIONS_DIR=/app/db/migrations" \
    -e "PUBLIC_BASE_URL=$PUBLIC_BASE_URL" \
    -e "TRUSTED_PROXY=$TRUSTED_PROXY" \
    -e PORT=8080 \
    "$PREVIOUS_IMAGE" serve
  if ! wait_healthy go-clipboard-prevcheck; then
    docker stop go-clipboard-prevcheck >/dev/null 2>&1 || true
    echo "previous image unhealthy against migrated schema; refusing release" >&2
    exit 1
  fi
  docker stop go-clipboard-prevcheck >/dev/null 2>&1 || true
fi

echo "starting health-gated candidate"
docker rm -f "$CANDIDATE_NAME" >/dev/null 2>&1 || true
docker run -d --name "$CANDIDATE_NAME" --rm \
  --network "$NETWORK" \
  -e "DATABASE_URL=$DATABASE_URL" \
  -e "MIGRATIONS_DIR=/app/db/migrations" \
  -e "PUBLIC_BASE_URL=$PUBLIC_BASE_URL" \
  -e "TRUSTED_PROXY=$TRUSTED_PROXY" \
  -e PORT=8080 \
  "$IMAGE" serve

if ! wait_healthy "$CANDIDATE_NAME"; then
  docker stop "$CANDIDATE_NAME" >/dev/null 2>&1 || true
  echo "candidate never became healthy; previous container keeps serving" >&2
  exit 1
fi

echo "swapping published app to $IMAGE"
docker stop "$CANDIDATE_NAME" >/dev/null 2>&1 || true
docker compose -f "$COMPOSE_DIR/compose.yaml" up -d --no-deps app

echo "$NEW_ID" >"$STATE_FILE"
echo "deploy complete: $IMAGE"
