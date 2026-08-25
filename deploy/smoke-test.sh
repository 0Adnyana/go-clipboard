#!/usr/bin/env bash
# Smoke-test a locally built image by digest/tag before publish.
set -euo pipefail

IMAGE="${1:?usage: smoke-test.sh <image-ref>}"
NETWORK=go-clipboard-smoke
PG_NAME=go-clipboard-smoke-pg
APP_NAME=go-clipboard-smoke-app
PASSWORD=smoke-test-password

cleanup() {
  docker rm -f "$APP_NAME" "$PG_NAME" "${PG_NAME}-empty" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Probe over TCP, not the default Unix socket: the entrypoint's init-only temp
# server runs with listen_addresses='' and answers a socket pg_isready, so a
# socket probe reports ready before the real server is listening on 5432.
wait_for_pg() {
  local container="$1" db="$2"
  for _ in $(seq 1 60); do
    if docker exec "$container" \
      pg_isready -h 127.0.0.1 -p 5432 -U clipboard -d "$db" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  echo "postgres container $container never accepted TCP connections" >&2
  docker logs "$container" >&2 || true
  return 1
}

docker network create "$NETWORK" >/dev/null
docker run -d --name "$PG_NAME" --network "$NETWORK" \
  -e POSTGRES_USER=clipboard \
  -e POSTGRES_PASSWORD="$PASSWORD" \
  -e POSTGRES_DB=go_clipboard \
  postgres:18-alpine >/dev/null

wait_for_pg "$PG_NAME" go_clipboard

DATABASE_URL="postgres://clipboard:${PASSWORD}@${PG_NAME}:5432/go_clipboard?sslmode=disable"

echo "migrate up"
docker run --rm --network "$NETWORK" \
  -e "DATABASE_URL=$DATABASE_URL" \
  "$IMAGE" migrate up

echo "migrate status"
docker run --rm --network "$NETWORK" \
  -e "DATABASE_URL=$DATABASE_URL" \
  "$IMAGE" migrate status

user="$(docker image inspect --format '{{.Config.User}}' "$IMAGE")"
if [[ "$user" != "nonroot" && "$user" != "nonroot:nonroot" && "$user" != "65532:65532" && "$user" != "65532" ]]; then
  echo "expected non-root image user, got: $user" >&2
  exit 1
fi
echo "runtime user: $user"

echo "serve (plain HTTP behind edge)"
docker run -d --name "$APP_NAME" --network "$NETWORK" \
  -e "DATABASE_URL=$DATABASE_URL" \
  -e PUBLIC_BASE_URL=https://localhost \
  -e TRUSTED_PROXY=127.0.0.1 \
  -e PORT=8080 \
  "$IMAGE" serve >/dev/null

ok=0
for _ in $(seq 1 30); do
  if docker run --rm --network "$NETWORK" curlimages/curl:8.5.0 \
    -fsS -H "Host: localhost" "http://${APP_NAME}:8080/api/health" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 1
done
if [[ "$ok" -ne 1 ]]; then
  echo "health check failed" >&2
  docker logs "$APP_NAME" >&2 || true
  exit 1
fi

echo "embedded frontend"
body="$(docker run --rm --network "$NETWORK" curlimages/curl:8.5.0 \
  -fsS -H "Host: localhost" "http://${APP_NAME}:8080/")"
echo "$body" | grep -qi '<!doctype html\|<html' || {
  echo "frontend not served: $body" >&2
  exit 1
}

echo "SPA fallback"
code="$(docker run --rm --network "$NETWORK" curlimages/curl:8.5.0 \
  -s -o /dev/null -w '%{http_code}' -H "Host: localhost" \
  "http://${APP_NAME}:8080/some-slug")"
[[ "$code" == "200" ]] || { echo "SPA fallback status=$code" >&2; exit 1; }

echo "pending-migration detectability"
# Drop goose version artificially? Instead run a fresh empty DB without migrate.
docker rm -f "$APP_NAME" >/dev/null
docker run -d --name "${PG_NAME}-empty" --network "$NETWORK" \
  -e POSTGRES_USER=clipboard \
  -e POSTGRES_PASSWORD="$PASSWORD" \
  -e POSTGRES_DB=empty_db \
  postgres:18-alpine >/dev/null
wait_for_pg "${PG_NAME}-empty" empty_db
docker run -d --name "$APP_NAME" --network "$NETWORK" \
  -e "DATABASE_URL=postgres://clipboard:${PASSWORD}@${PG_NAME}-empty:5432/empty_db?sslmode=disable" \
  -e PUBLIC_BASE_URL=https://localhost \
  -e TRUSTED_PROXY=127.0.0.1 \
  -e PORT=8080 \
  "$IMAGE" serve >/dev/null
pending_code="000"
for _ in $(seq 1 30); do
  pending_code="$(docker run --rm --network "$NETWORK" curlimages/curl:8.5.0 \
    -s -o /tmp/health.json -w '%{http_code}' -H "Host: localhost" \
    "http://${APP_NAME}:8080/api/health" || true)"
  [[ "$pending_code" != "000" ]] && break
  sleep 1
done
docker rm -f "${PG_NAME}-empty" >/dev/null 2>&1 || true
[[ "$pending_code" == "503" ]] || {
  echo "expected 503 with pending migrations, got $pending_code" >&2
  exit 1
}

echo "smoke test passed for $IMAGE"
