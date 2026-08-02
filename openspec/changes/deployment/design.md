## Context

Slices 1–3 are a local milestone. This slice (see `docs/slices/04-deployment.md`) delivers the machinery to run the service somewhere that is not a laptop, with an automatic path from commit to running service. It is deliberately positioned last-before-secrets: slice 6 needs an absolute base URL and mail credentials, slice 11 needs a client secret and a redirect URI against a real hostname. It is explicitly *not* an exposure — slice 12 remains the publishability gate.

Current state the design builds on:

- The server binary already has `migrate up` / `migrate status` subcommands and never migrates on startup (`schema-migrations`).
- `GET /api/health` reports pending migrations and DB reachability (`persistence`, `http-server`).
- Configuration is typed and read from the environment; the server does not parse `.env` itself (`http-server`).
- A dev `Caddyfile` proxies `/api/*` to Go and everything else to Vite; static delivery, SPA fallback, and the `/<slug>` catch-all are currently *Vite's* behaviour, not written config (`edge-proxy`). Production embeds the frontend in the Go binary and serves it in-process over plain HTTP; TLS stays at a separate public edge.
- `make test` / `make lint` cover `go test`, `gofmt`, `go vet`, and the frontend typecheck/lint. `sqlc generate` produces committed Go from `db/queries`.

The slice doc leaves several things "still undecided". This design settles them with reasoned defaults to keep momentum; each is listed under Decisions with the alternative and is cheap to revisit before implementation.

## Goals / Non-Goals

**Goals:**

- A multi-stage `Dockerfile` producing one immutable, SHA-tagged image per commit carrying a single server binary (the built frontend embedded via `go:embed`) and the migration runner.
- The Go binary serving production behind an edge TLS terminator: plain HTTP, embedded assets with SPA fallback, and `/api/*` by matcher specificity in-process.
- A CI pipeline that gates (test, lint, sqlc drift, image build) on every commit and, on the default branch, publishes and deploys with migrations as an explicit step.
- Rollback by re-pointing to a previous SHA-tagged image.
- Environment-based configuration with secrets outside repo and image; adds the public base URL and a trusted edge hop for rate limiting.

**Non-Goals:**

- Telling anyone the address (slice 12), a staging environment, per-PR previews, blue/green.
- A second instance, a load balancer, or zero-downtime swap — a deploy is a brief outage.
- Containerising Postgres for development, or the dev loop at all.
- Metrics, tracing, alerting, uptime checks, log aggregation.
- Secret rotation; automatic migrations anywhere; cron/workers/queue.
- Shipping or managing the public edge Caddy configuration in this repository.

## Decisions

### D1. Where it runs: Compose on a private host behind a public edge

The app and Postgres run via Docker Compose on a private host (homelab / Portainer). A separate public edge (Caddy on a VPS) terminates TLS and reaches the app over a private path (Tailscale). Chosen: **edge TLS + private Compose**. This keeps certificates and public exposure at the edge the operator already runs, while the image CI tested is still what serves API and UI.

- *Alternative (single VPS, Go terminates TLS with autocert):* fewer moving parts on paper, but duplicates TLS when an edge already exists and forces ACME reachability onto the app host. Rejected for this topology.
- *Alternative (managed platform owning the origin):* less host toil, but cedes more of the stack. Rejected for now.

### D1a. No in-process TLS; the Go binary serves plain HTTP with embedded assets

The dev `Caddyfile` (`edge-proxy`) stays exactly as it is. Production does not run Caddy beside the app: the Go server serves the frontend from assets embedded with `//go:embed` and routes `/api/*` in-process over plain HTTP. TLS and ACME stay at the public edge. Chosen: **serve in-process over HTTP behind an edge**.

- *Alternative (production Caddy beside the app serving static files):* mirrors dev one-to-one but keeps a second container and leaves frontend bytes outside the image. Rejected in favour of embed.
- *Alternative (autocert inside Go):* rejected per D1 when a TLS-terminating edge is already present.

### D2. Production Postgres: container beside the app, with a volume

This is the one place the overview's Docker rejection genuinely reopens — its premise (a machine already running the target version) is false on a fresh host. Chosen: **a Postgres container on the same box** with a named volume, defined in the same compose file. Postgres stays on a **private Compose network reachable only by the application and migration containers**: its port is never published to the host. Managed Postgres stays the easy upgrade path if durability needs rise.

### D3. Registry: Docker Hub, public image

Chosen: **Docker Hub, public**. Each build pushes the **full commit SHA** tag and a moving **`latest`** alias. `latest` is never a deploy target.

### D4. Deploy is pushed from CI over SSH

Chosen: **push** — CI holds host credentials and, over SSH, pulls the SHA-tagged image on the host, runs `<image> migrate up` against `DATABASE_URL`, and only then swaps the serving container with `docker compose up -d`.

### D5. Backups: nightly `pg_dump` of non-ephemeral data

Everything in `clips` dies within 24h, so backups target only accounts and (from slice 12) the audit log. A restore that loses every live clip costs ~nothing.

### D6. Health-gated swap: yes

The new image is first started as a candidate on the private Compose network; the deploy polls `GET /api/health` (with the configured `Host`) until healthy, then swaps the published app. If the candidate never becomes healthy, the previous container keeps serving.

### D7. Migrations run as an explicit deploy step, before the swap

Never on container start. Backward-compatible-migration discipline makes this ordering safe with a brief single-instance outage.

### D8. Hostname: a single configured hostname via env

One production hostname via `PUBLIC_BASE_URL` (the edge-facing HTTPS origin). The Go server enforces that `Host` matches the derived hostname.

### D9. Trusted forwarded hop: the edge proxy

`TRUSTED_PROXY` is the edge's IP as seen by the app (e.g. Tailscale address). `X-Forwarded-For` from that peer is honoured for rate limiting; other peers are not trusted.

## Risks / Trade-offs

- **Single instance is load-bearing** → Stated as an explicit constraint in `production-runtime`.
- **Backward-incompatible migration + rollback breaks** → Rollback re-points the image but never migrates down.
- **DB disk = app disk (D2)** → Mitigation: D5 backups off-box.
- **CI holds production credentials (D4)** → Mitigation: least-privilege deploy key.
- **Edge is outside this repo** → Misconfigured Caddy/`TRUSTED_PROXY` breaks TLS or client identity; documented in `deploy/env.example`.
- **Brief 502 on every deploy** → accepted for a clipboard; D6 fails safe.

## Migration Plan

1. Provision the private Compose host and ensure the public edge can reach `:8080` over the private path; point DNS at the edge.
2. Land the embedded-assets package and plain-HTTP serving wiring in Go, then the `Dockerfile`, `compose.yaml`, and CI workflow.
3. First deploy: push image, run `migrate up`, bring up the app with `TRUSTED_PROXY` set to the edge IP, verify `GET /api/health` through the edge hostname over TLS.
4. **Rollback:** redeploy the previous SHA tag; do not migrate down.

## Open Questions

- Exact host sizing and the concrete hostname — operational, not architectural.
- Whether managed Postgres is adopted before durability needs actually rise.
- The Docker Hub namespace/repository name for the public image.
