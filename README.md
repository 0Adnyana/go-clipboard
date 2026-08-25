# go-clipboard

Online clipboard built with Go, inspired by [cl1p](https://cl1p.net). Local development covers paste-and-read clips; production packaging (container image, edge-terminated TLS, CI deploy) is in place. Public exposure remains gated by a later slice.

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

The create form should load at `/`. Paste text under a name, then open `http://localhost:3000/<name>` in another browser context to read it back. Stack diagnostics remain at `/status` once `make migrate` has succeeded.

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
curl -s -X POST http://localhost:3000/api/clips \
  -H 'Content-Type: application/json' \
  -d '{"slug":"demo-clip","body":"hello\n"}' | jq
curl -s http://localhost:3000/api/clips/demo-clip | jq
```

In development, Go serves `/api/*` behind Caddy; Vite owns the UI. In production the Go binary serves the embedded frontend (SPA fallback) and `/api/*` over plain HTTP behind an edge that terminates TLS.

Product intent stays in [`docs/slices/`](docs/slices/); grow the OpenAPI file in the same change as each new handler.

## Deployment

Production runs as Docker Compose on a private LXC (e.g. homelab via Portainer), reached from a public edge proxy (e.g. Caddy on a VPS) through a Tailscale **subnet router** onto the LAN. The edge owns TLS; the app serves plain HTTP on `:8080` with the embedded frontend and API. Exactly one app instance is required (sweeper, deployment locking, in-process limiter). Setup progress is the checklist in [`DEPLOYMENT.md`](DEPLOYMENT.md#setup-checklist).

### Artefacts

| Path | Role |
| --- | --- |
| `Dockerfile` | Multi-stage image: `pnpm build` → Go embed → distroless non-root binary |
| `compose.yaml` | App (`8080`) + Postgres |
| `.github/workflows/ci.yml` | Gate (`make test`/`lint`, sqlc drift, image build + smoke) on every commit; publish + SSH deploy on `main` |
| `deploy/deploy.sh` | Host-side migrate → health-gated swap; rejects `latest` |
| `deploy/backup.sh` / `deploy/restore.sh` | Nightly `pg_dump` excluding ephemeral `clips` |

### Required secrets / env

**CI (GitHub Actions secrets):** `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`, `DEPLOY_HOST` (LXC LAN IP), `DEPLOY_USER`, `DEPLOY_SSH_KEY`, `TS_OAUTH_CLIENT_ID`, `TS_OAUTH_SECRET`. Pin that same LAN IP in committed `deploy/known_hosts` (never `StrictHostKeyChecking=no`). The deploy job joins Tailscale (`tag:ci`, `--accept-routes`) before SSH.

**Host env file** (next to `compose.yaml`, from `deploy/env.example`): `PUBLIC_BASE_URL` (absolute HTTPS origin at the edge), `POSTGRES_PASSWORD`, `TRUSTED_PROXY` (edge proxy IP as seen by the app). `IMAGE` is set by the deploy script to a content digest (`registry/name@sha256:…`) — never `latest`.

### Rollback

Redeploy the previous digest with `deploy/deploy.sh <previous-image-ref>` only — do not swap the app container by hand. Do **not** migrate down. Each release runs the deployment script’s compatibility checks (previous image healthy against the migrated schema) before swapping.

### Health contract

`GET /api/health` returns **200** only when the database is reachable and migrations are not pending; otherwise **503**. That confirms database reachability and migration state, not that application queries are compatible with the migrated schema — rely on the deployment script’s compatibility checks for that. A half-landed deploy (new image, un-migrated schema) is still detectable by status code alone.

## Gotchas

### TanStack Router plugin order

In `web/vite.config.ts`, register `tanstackRouter({ target: 'react' })` **before** `react()`. Reversing the order breaks route generation silently.

### Gateway 502 errors

- **502 on `/api/*`** — the Go server on `:8080` is down (often during an `air` restart; Caddy retries for up to 5s).
- **502 on page loads** — the Vite dev server on `:5173` is down.

### Migrations are deliberate

The server never applies migrations on startup. Run `make migrate` locally, or `migrate up` via the image as an explicit deploy step. `/api/health` returns 503 while migrations are pending (and when the database is unreachable).

### Homebrew `DATABASE_URL`

Copy `.env.example`, not a generic `postgres:postgres@localhost` URL, and substitute your own account name for `<macos-username>`. Homebrew creates a superuser named after your macOS account and typically uses trust auth on the local socket (`host=/tmp`) while TCP to `localhost` still wants a password.

### `/api/health` omits `migrations` when it cannot check

The health endpoint only reports what it observed. If the pending-migration check fails — usually because Postgres is unreachable — the `migrations` block is left out of the response entirely rather than defaulting to "not pending, version 0", and the status page shows the state as unknown.

## Architecture

**Development**

```text
browser → Caddy :3000 ─┬─ /api/* → Go :8080 → Postgres
                       └─ /*     → Vite :5173 (HMR)
```

**Production**

```text
browser → edge Caddy (TLS) → Tailscale → subnet router
       → LAN Compose LXC, Go :8080 ─┬─ /api/* → handlers → Postgres
                                    └─ /*     → embedded SPA (go:embed)
```