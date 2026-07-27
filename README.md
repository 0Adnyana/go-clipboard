# go-clipboard

Online clipboard built with Go, inspired by [cl1p](https://cl1p.net). This repository currently ships a **development walking skeleton**: a React status page, a JSON API, Postgres connectivity, and explicit schema migrations — all behind a single Caddy origin.

## Prerequisites

Install these before the first run:

| Tool | Version tested | Install |
| --- | --- | --- |
| Postgres | 18.x (`postgresql@18`) | `brew install postgresql@18` |
| Go | 1.25+ | `brew install go` |
| Node.js | 22.x | `brew install node` |
| pnpm | 11.x | `npm install -g pnpm` |
| Caddy | 2.x | `brew install caddy` |
| air | latest | `go install github.com/air-verse/air@latest` |
| sqlc | latest | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` |
| goose | latest | `go install github.com/pressly/goose/v3/cmd/goose@latest` |

Start Postgres locally (`brew services start postgresql@18`) and keep it running. The project consumes `DATABASE_URL` only; it does not start or containerise Postgres.

## Setup

From a clean clone:

```bash
cp .env.example .env
make db-create
make migrate
make generate
cd web && pnpm install && cd ..
make dev
```

After `cp .env.example .env`, edit `DATABASE_URL` and replace `<macos-username>` with your own account name (`whoami`) — it ships as a literal placeholder so the file is not tied to one machine.

Open the URL printed by `make dev` — **`http://localhost:3000`** (or your `PROXY_PORT`). That proxy port is the supported entry point. Ports `:8080` (Go) and `:5173` (Vite) bypass the proxy and are not supported for day-to-day development.

The status page should report the database reachable and no migrations pending once `make migrate` has succeeded.

## Common commands

```bash
make help        # list targets
make dev         # Caddy + air + Vite
make migrate     # apply pending migrations
make generate    # regenerate sqlc code
make db-reset    # drop, recreate, migrate
make test        # Go tests + frontend typecheck
make lint        # go vet + oxlint
```

## API

The HTTP contract lives in [`docs/api/openapi.yaml`](docs/api/openapi.yaml). Import that file into Postman (Import → OpenAPI), or call endpoints with curl against the Caddy origin:

```bash
curl -s http://localhost:3000/api/health | jq
```

Go serves only `/api/*`. Product intent stays in [`docs/slices/`](docs/slices/); grow the OpenAPI file in the same change as each new handler.

## Gotchas

### TanStack Router plugin order

In `web/vite.config.ts`, register `tanstackRouter({ target: 'react' })` **before** `react()`. Reversing the order breaks route generation silently.

### Gateway 502 errors

- **502 on `/api/*`** — the Go server on `:8080` is down (often during an `air` restart; Caddy retries for up to 5s).
- **502 on page loads** — the Vite dev server on `:5173` is down.

### Migrations are deliberate

The server never applies migrations on startup. Run `make migrate` after adding migration files. `/api/health` reports pending migrations instead of failing mysteriously at query time.

### Homebrew `DATABASE_URL`

Copy `.env.example`, not a generic `postgres:postgres@localhost` URL, and substitute your own account name for `<macos-username>`. Homebrew creates a superuser named after your macOS account and typically uses trust auth on the local socket (`host=/tmp`) while TCP to `localhost` still wants a password.

### `/api/health` omits `migrations` when it cannot check

The health endpoint only reports what it observed. If the pending-migration check fails — usually because Postgres is unreachable — the `migrations` block is left out of the response entirely rather than defaulting to "not pending, version 0", and the status page shows the state as unknown.

## Architecture (development)

```
browser → Caddy :3000 ─┬─ /api/* → Go :8080 → Postgres
                       └─ /*     → Vite :5173 (HMR)
```

Go serves **only** `/api/*`. Static assets and SPA routing stay in the frontend/proxy layer.
