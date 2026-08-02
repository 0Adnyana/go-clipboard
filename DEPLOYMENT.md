# Deployment

Operational runbook for running go-clipboard in production. Product intent for this slice lives in [`docs/slices/04-deployment.md`](docs/slices/04-deployment.md); design decisions in [`openspec/changes/deployment/design.md`](openspec/changes/deployment/design.md).

A deployment is **not** a public exposure — publishing the address remains a later gate. Exactly **one** app instance is required (in-process sweeper, migration locking, rate-limiter state).

## Topology

```text
browser → edge Caddy (TLS / ACME) → private path (e.g. Tailscale)
       → Compose host :8080 (Go, plain HTTP)
            ├─ /api/*  → handlers → Postgres (Compose network only)
            └─ /*      → embedded SPA (go:embed)
```

| Piece | Where | Notes |
| --- | --- | --- |
| TLS / public hostname | Public edge (Caddy on a VPS) | Not in this repo; not terminated by Go |
| App + Postgres | Private host (Compose / Portainer) | Image from Docker Hub; Postgres port unpublished |
| Path edge → app | Tailscale / LAN | Edge IP becomes `TRUSTED_PROXY` |

## Artefacts

| Path | Role |
| --- | --- |
| [`Dockerfile`](Dockerfile) | Multi-stage: `pnpm build` → Go embed → distroless non-root binary + migrations |
| [`compose.yaml`](compose.yaml) | App (`8080`) + Postgres 18 (named volume `pgdata`) |
| [`.github/workflows/ci.yml`](.github/workflows/ci.yml) | Gate on every commit; publish + SSH deploy on `main` |
| [`deploy/deploy.sh`](deploy/deploy.sh) | Host-side: migrate → compatibility check → health-gated swap; rejects `latest` |
| [`deploy/smoke-test.sh`](deploy/smoke-test.sh) | CI local smoke (migrate, non-root user, health, SPA) |
| [`deploy/backup.sh`](deploy/backup.sh) / [`deploy/restore.sh`](deploy/restore.sh) | Nightly dump (excludes `clips` data) and restore |
| [`deploy/env.example`](deploy/env.example) | Template for the host `.env` |
| [`deploy/known_hosts`](deploy/known_hosts) | Pinned production SSH host keys (required before CI deploy) |
| [`deploy/crontab.example`](deploy/crontab.example) | Nightly backup cron |

## First-time host setup

1. Provision a private host with Docker and Compose. Default deploy directory: `/opt/go-clipboard`.
2. Ensure the public edge can reach `host:8080` over the private path; point DNS at the edge.
3. Copy Compose and scripts (CI also syncs these on each deploy):

   ```bash
   mkdir -p /opt/go-clipboard
   # place compose.yaml, deploy.sh, backup.sh, restore.sh here
   chmod +x /opt/go-clipboard/deploy.sh /opt/go-clipboard/backup.sh /opt/go-clipboard/restore.sh
   ```

4. Create the host env file next to `compose.yaml`:

   ```bash
   cp deploy/env.example /opt/go-clipboard/.env
   # edit PUBLIC_BASE_URL, POSTGRES_PASSWORD, TRUSTED_PROXY
   ```

5. Pin the host’s SSH host key into committed [`deploy/known_hosts`](deploy/known_hosts) (never `StrictHostKeyChecking=no`).
6. Configure GitHub Actions secrets and the `production` environment (see below).
7. First deploy: push to `main` (or run `deploy.sh` manually with a SHA-tagged image). Verify `GET https://<hostname>/api/health` through the edge.

Optional: install nightly backups from [`deploy/crontab.example`](deploy/crontab.example) and set `BACKUP_REMOTE`.

## Configuration

### Host env (`/opt/go-clipboard/.env`)

From [`deploy/env.example`](deploy/env.example). Secrets stay on the host — never in the repo or image.

| Variable | Required | Purpose |
| --- | --- | --- |
| `PUBLIC_BASE_URL` | yes | Absolute HTTPS origin at the edge (no path/query/fragment). Server derives `Host` from this. |
| `POSTGRES_PASSWORD` | yes | Compose Postgres password; interpolated into `DATABASE_URL` |
| `TRUSTED_PROXY` | yes | Edge proxy IP as seen by the app (Tailscale/LAN). Honours `X-Forwarded-For` only from this peer. |
| `IMAGE` | set by deploy | Digest or full 40-char commit SHA tag — **never** `latest` |

`DATABASE_URL` is composed by `compose.yaml` / `deploy.sh` against the private `postgres` service. Do not publish Postgres to the host.

### GitHub Actions secrets

| Secret | Purpose |
| --- | --- |
| `DOCKERHUB_USERNAME` | Registry namespace (`docker.io/<user>/go-clipboard`) |
| `DOCKERHUB_TOKEN` | Push credentials |
| `DEPLOY_HOST` | Production SSH hostname/IP |
| `DEPLOY_USER` | SSH user |
| `DEPLOY_SSH_KEY` | Private key (least-privilege deploy key) |

Workflow env: `REGISTRY=docker.io`, `IMAGE_NAME=${{ secrets.DOCKERHUB_USERNAME }}/go-clipboard`. Deploys use the `production` GitHub Environment.

## CI / CD

On every commit / PR:

1. **Gate** — `make test`, `make lint`, `sqlc generate` clean-tree check
2. **Image** — build Dockerfile, run `deploy/smoke-test.sh`

On push to `main` only (after gate + image):

1. Push tags: `<repo>:<full-git-sha>` and `<repo>:latest` (`latest` is convenience only)
2. Deploy target is the **SHA tag** (also reject `latest` in CI and in `deploy.sh`)
3. SCP `compose.yaml` + deploy/backup/restore scripts to `/opt/go-clipboard`
4. SSH: `deploy.sh <sha-image> [previous-sha-image]`

Deploys are serialized (`concurrency: production-deploy`).

## Deploy flow (`deploy/deploy.sh`)

```text
flock → up postgres → pull IMAGE → migrate up
  → (optional) previous image healthy against migrated schema
  → candidate container on Compose network → poll GET /api/health
  → compose up -d app → write deployed-sha
```

- Migrations run as an explicit `docker run … <image> migrate up` step — **never** on container start.
- Candidate must return healthy health within `HEALTH_TIMEOUT_SEC` (default 90s) or the previous container keeps serving.
- `EXPECTED_PREVIOUS_SHA` (set by CI) refuses to clobber an unexpected live deployment.
- Brief outage during the published swap is expected (single instance).

Manual rollback / redeploy on the host:

```bash
cd /opt/go-clipboard
./deploy.sh docker.io/<user>/go-clipboard:<previous-40-char-sha>
```

## Rollback

1. Redeploy the previous SHA (or digest) with `deploy.sh` only — do not swap the app container by hand.
2. **Do not migrate down.** Rollback re-points the image; schema stays forward.
3. Each release runs a compatibility gate: after `migrate up`, the **previous** image must still serve healthy against the migrated schema, or the release is refused.

## Health contract

`GET /api/health` with `Host` matching the hostname from `PUBLIC_BASE_URL`:

| Outcome | Status | Meaning |
| --- | --- | --- |
| DB reachable, no pending migrations | **200** | Deploy health gate passes |
| Pending migrations or DB error | **503** | Unhealthy; swap does not complete |

Health confirms DB reachability and migration state, not full query compatibility with a new schema — that is why the previous-image compatibility check exists.

Through the edge after deploy:

```bash
curl -sS "https://clip.example.com/api/health" | jq
```

## Backups and restore

Clips expire within ~24h; dumps exclude table data for `clips`. Losing live clips on restore is accepted.

```bash
# Nightly (see deploy/crontab.example)
COMPOSE_DIR=/opt/go-clipboard BACKUP_REMOTE=user@backup-host:/backups/go-clipboard \
  /opt/go-clipboard/backup.sh

# Restore
/opt/go-clipboard/restore.sh /var/backups/go-clipboard/go-clipboard-YYYYMMDDTHHMMSSZ.sql.gz
```

Local dumps under `/var/backups/go-clipboard` are retained 7 days; set `BACKUP_REMOTE` to rsync off-box.

## Edge proxy (outside this repo)

The edge must:

- Terminate TLS for `PUBLIC_BASE_URL`’s hostname
- Proxy to the app over the private path on port `8080`
- Preserve/forward client identity so the app’s `TRUSTED_PROXY` (edge IP) can trust `X-Forwarded-For`

Misconfigured `TRUSTED_PROXY` breaks rate-limit client identity; wrong `Host` / `PUBLIC_BASE_URL` causes the app to reject requests.

## Constraints

- **One app replica** — do not scale the Compose `app` service.
- **No auto-migrate on start** — only `migrate up` via the image during deploy.
- **No `latest` deploys** — SHA tag or digest only.
- **Secrets outside image** — rotate by editing host `.env` / CI secrets; no rebuild required for secret-only changes.
- **Logs** — stdout/stderr only; collected by the container runtime.
- **Out of scope here** — staging, blue/green, metrics/alerting, managed Postgres (easy upgrade later), shipping the public edge Caddyfile in-repo.
