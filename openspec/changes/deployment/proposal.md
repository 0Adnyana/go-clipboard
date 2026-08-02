## Why

Slices 1–3 are a local milestone with nothing allowed to leave the laptop, but every remaining slice arrives holding a secret or a public URL (slice 6 needs credentials and an absolute base URL; slice 11 needs a client secret and a registered redirect URI against a real hostname). This is the last position where deployment is new infrastructure rather than a migration of a running service, so the project needs somewhere to run that is not a laptop and an automatic path from commit to running service — without weakening slice 12, which stays the publishability gate. A deployment is not an exposure.

## What Changes

- The application is containerised as a **deployment artefact only**; the development loop (`make dev`, `air`, Homebrew Postgres) is untouched and Postgres is never containerised for development.
- **One image per commit**, built once by CI via a multi-stage build (`pnpm build` then Go build with the built assets embedded via `go:embed`, small runtime carrying the single server binary and the migration runner out), tagged by commit SHA. `latest` is refused as a deploy target.
- **Production serving moves into the Go binary behind an edge TLS terminator.** The server serves plain HTTP with the embedded frontend (SPA fallback to `index.html`) and routes `/api/*` in-process by matcher specificity. A separate edge (e.g. Caddy on a public VPS) terminates TLS and forwards over a private path (e.g. Tailscale) to the Compose host. The dev `Caddyfile` (`edge-proxy`) is untouched. `TRUSTED_PROXY` is the edge hop so rate limiting uses the real client IP from `X-Forwarded-For`.
- **Migrations are a pipeline step**, run through the same image with a different argument, never on container start. The health endpoint's pending-migration report becomes the check that catches a half-landed deploy.
- **Configuration is environment; secrets live in neither the repository nor the image.** Adds the public base URL (for slice 6 reset links) and requires `TRUSTED_PROXY` in production as the edge proxy IP.
- A **CI pipeline that first refuses**: `make test`, `make lint`, plus a `sqlc generate` clean-tree drift check and building the image on every commit. On `main`, it publishes and deploys.
- **Rollback is re-pointing to the previous image tag** — the first thing to reach for. Each migration's backward compatibility with the prior image is an **enforced release gate**, not an unenforced discipline: the deploy applies the migration and then verifies the previous SHA image starts and reports healthy against the newly-migrated schema, failing the release if it does not, so a migration that would break rollback is caught before it ships.
- **Structured logs to stdout** are the entire observability story; **exactly one instance** is stated as an explicit constraint (a deploy is a brief outage and discards in-process limiter state on every push).

## Capabilities

### New Capabilities

- `container-image`: The production container image — multi-stage build producing one immutable image per commit tagged by SHA, carrying a single server binary (frontend assets embedded via `go:embed`) and the migration runner; `latest` refused as a deploy target.
- `production-serving`: In-process production serving by the Go binary — plain HTTP behind an edge TLS terminator, embedded frontend with SPA fallback to `index.html`, and `/api/*` by matcher specificity.
- `deployment-pipeline`: The CI/CD pipeline — gating jobs (test, lint, sqlc drift check, image build) that refuse before anything ships, image publish to the registry on `main`, a deploy step that applies migrations then swaps the running image behind a mandatory health-gated swap (design decision D6), and rollback by re-pointing to the previous tag.
- `production-runtime`: The production runtime environment — a single instance as an explicit constraint, environment-based configuration with secrets outside the repo and image, the public base URL and a trusted edge hop, structured logs to stdout, a mandatory health-gated swap (design decision D6), and backups scoped to non-ephemeral data.

### Modified Capabilities

<!-- No existing spec-level requirements change. schema-migrations already mandates explicit, invoked-only migrations; running them as a pipeline step is consistent with it, not a requirement change. edge-proxy remains the development origin; production TLS lives at a separate edge in front of the Compose app. -->

## Impact

- **New files (mostly configuration — the project's convention is unit tests only):** a `Dockerfile` (multi-stage), a deployment `compose.yaml`, and a CI workflow. **The one exception is Go code:** a small embedded-assets package (`//go:embed` of the built frontend) plus wiring so the server serves static/SPA fallback over plain HTTP.
- **Configuration:** a new environment variable for the public base URL; the trusted forwarded hop set to the edge proxy IP in production. A secret store outside the repo and image.
- **Existing behaviour relied on:** the `migrate up` / `migrate status` subcommands on the server binary, the `GET /api/health` pending-migration report, `make test` / `make lint`, and `sqlc generate`.
- **Registry:** a public Docker Hub image repository; the app host publishes plain HTTP (e.g. `:8080`) on a private path; the public edge owns ports 80/443 and TLS.
- **Carried-forward debt:** single-instance assumptions (no advisory lock, one sweeper, in-process limiter state) are stated, not solved; a deploy is a brief outage.
