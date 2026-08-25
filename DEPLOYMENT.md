# Deployment

Operational runbook for running go-clipboard in production. Product intent for this slice lives in [`docs/slices/04-deployment.md`](docs/slices/04-deployment.md); design decisions in [`openspec/changes/deployment/design.md`](openspec/changes/deployment/design.md).

A deployment is **not** a public exposure — publishing the address remains a later gate. Exactly **one** app instance is required (in-process sweeper, migration locking, rate-limiter state).

## Topology

This is **edge TLS termination in front of a private origin**: a public reverse proxy terminates TLS; the app runs on a private Compose host (Docker LXC) that is **not** on Tailscale. A homelab **subnet router** advertises the LAN (`192.168.0.0/24`) so the edge and CI reach `192.168.0.202` through Tailscale. Releases are **push-based CD** — GitHub Actions joins the tailnet (with subnet routes), SSHs to that LAN address, and runs [`deploy/deploy.sh`](deploy/deploy.sh). That is distinct from pull-based / GitOps (Portainer Git sync, watch `latest`).

```text
browser → edge Caddy (TLS / ACME)
       → Tailscale → subnet router (tag:homelab)
       → LAN 192.168.0.202:8080  Compose LXC (Go, plain HTTP; no Tailscale)
            ├─ /api/*  → handlers → Postgres (Compose network only)
            └─ /*      → embedded SPA (go:embed)

GitHub runner (tag:ci, --accept-routes)
       → Tailscale → subnet router → 192.168.0.202:22
```

| Piece                   | Where                           | Notes                                                                             |
| ----------------------- | ------------------------------- | --------------------------------------------------------------------------------- |
| TLS / public hostname   | Public edge (Caddy on a VPS)    | Not in this repo; not terminated by Go                                            |
| App + Postgres          | Compose LXC at `192.168.0.202`  | No Tailscale on this host; image from Docker Hub; Postgres unpublished            |
| Subnet router           | Homelab node on the tailnet     | Tagged `tag:homelab`; advertises `192.168.0.0/24` (approved in the admin console) |
| Path edge → app `:8080` | Tailscale → subnet → LAN        | TCP peer as seen by Go becomes `TRUSTED_PROXY`                                    |
| Path CI → host `:22`    | Tailscale → subnet → LAN        | Separate from the app path; see [CI over Tailscale](#ci-over-tailscale)           |
| Portainer               | Optional UI on the Compose host | Watches the stack; does not deploy it                                             |

Do not keep a git checkout on the production host. The running artefact is the digest-pinned image; CI copies only `compose.yaml` and the deploy scripts into `/opt/go-clipboard`. Host secrets live in `/opt/go-clipboard/.env` (from [`deploy/env.example`](deploy/env.example)), never in the repo or image. The laptop `.env.example` is for `make dev` only — do not copy it onto the host.

## Setup checklist

Operator progress for this homelab. Details for each item are in [First-time host setup](#first-time-host-setup) and [CI over Tailscale](#ci-over-tailscale).

### Tailscale

- [x] `tagOwners` for `tag:ci` and `tag:homelab`
- [x] Tag **only** the subnet router `tag:homelab` (not the LXC; drop `tag:ci` from any always-on node — runners get it at job time)
- [x] Default `*` grant removed; `tag:ci` → `192.168.0.0/24` on `tcp:22`
- [x] Member and `tag:homelab` grants also include `192.168.0.0/24` (required once `*` is gone, or laptops/edge lose the LAN)
- [x] ACL `tests`: `tag:ci` accept `192.168.0.202:22`, deny `:8080` and `:5432`
- [x] OAuth client (write **Devices Core** + **Auth Keys**, tags field `tag:ci` only)
- [x] Subnet route `192.168.0.0/24` approved on the router in the admin console

### GitHub / repo

- [x] Actions secrets: `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`, `DEPLOY_HOST` (`192.168.0.202`), `DEPLOY_USER`, `DEPLOY_SSH_KEY`, `TS_OAUTH_CLIENT_ID`, `TS_OAUTH_SECRET`
- [x] GitHub Environment named `production` (the deploy job requires it)
- [x] [`deploy/known_hosts`](deploy/known_hosts) pins `192.168.0.202` (same string as `DEPLOY_HOST`)
- [x] Workflow Tailscale step has `args: --accept-routes` (commit this before the first deploy)

### Compose host (LXC)

- [x] Deploy key’s public half in `~/.ssh/authorized_keys` for `DEPLOY_USER`
- [ ] `/opt/go-clipboard` bootstrapped (`compose.yaml`, scripts, `.env` from `deploy/env.example`)
- [ ] LXC `:22` not published on the public internet

### First deploy

- [ ] Push to `main` (includes `--accept-routes`) so `publish-and-deploy` runs
- [ ] `GET https://<hostname>/api/health` through the edge returns 200

## Usual setup: CI deploys, Portainer watches

1. Bootstrap `/opt/go-clipboard` and `.env` on the host (below).
2. Let GitHub Actions SSH in and run `deploy.sh` on push to `main`.
3. In Portainer **Stacks**, `go-clipboard` appears as an existing Compose project. Use Portainer for logs, inspect, and a one-off restart.

Do **not** recreate that stack from Git in Portainer. Do not let a Portainer Git stack and `deploy.sh` both run `compose up` on the same project. Do not scale `app`, auto-update from `latest`, or skip `migrate up` (Portainer “Deploy” is not a full release — migrations never run on container start).

## Artefacts

| Path                                                                              | Role                                                                           |
| --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| [`Dockerfile`](Dockerfile)                                                        | Multi-stage: `pnpm build` → Go embed → distroless non-root binary + migrations |
| [`compose.yaml`](compose.yaml)                                                    | App (`8080`) + Postgres 18 (named volume `pgdata`)                             |
| [`.github/workflows/ci.yml`](.github/workflows/ci.yml)                            | Gate on every commit; publish + SSH deploy on `main`                           |
| [`deploy/deploy.sh`](deploy/deploy.sh)                                            | Host-side: migrate → compatibility check → health-gated swap; rejects `latest` |
| [`deploy/smoke-test.sh`](deploy/smoke-test.sh)                                    | CI local smoke (migrate, non-root user, health, SPA, 503 while pending)        |
| [`deploy/backup.sh`](deploy/backup.sh) / [`deploy/restore.sh`](deploy/restore.sh) | Nightly dump (excludes `clips` data) and restore                               |
| [`deploy/env.example`](deploy/env.example)                                        | Template for the host `.env`                                                   |
| [`deploy/known_hosts`](deploy/known_hosts)                                        | Pinned production SSH host keys (required before CI deploy)                    |
| [`deploy/crontab.example`](deploy/crontab.example)                                | Nightly backup cron                                                            |

## First-time host setup

1. Provision a private host with Docker and Compose. Default deploy directory: `/opt/go-clipboard`.
2. Ensure the public edge can reach `192.168.0.202:8080` (LAN or Tailscale → subnet router); point DNS at the edge.
3. Bootstrap Compose and scripts from a **scratch clone** (CI also syncs these on each deploy). Do not leave a working tree on the host:

   ```bash
   git clone https://github.com/0adnyana/go-clipboard.git /tmp/go-clipboard
   cd /tmp/go-clipboard

   sudo mkdir -p /opt/go-clipboard
   sudo cp compose.yaml deploy/deploy.sh deploy/backup.sh deploy/restore.sh /opt/go-clipboard/
   sudo chmod +x /opt/go-clipboard/deploy.sh /opt/go-clipboard/backup.sh /opt/go-clipboard/restore.sh

   sudo cp deploy/env.example /opt/go-clipboard/.env
   sudo chmod 600 /opt/go-clipboard/.env
   # edit PUBLIC_BASE_URL, POSTGRES_PASSWORD, TRUSTED_PROXY

   rm -rf /tmp/go-clipboard
   ```

   Later deploys overwrite Compose and scripts via SCP; they do **not** touch `.env`.

4. Pin the host’s SSH host key into committed [`deploy/known_hosts`](deploy/known_hosts) (never `StrictHostKeyChecking=no`). Scan the same address CI will use — the LXC LAN IP (`192.168.0.202`), not a MagicDNS / `100.x` name (this host has none). See [CI over Tailscale](#ci-over-tailscale).
5. Configure GitHub Actions secrets and the `production` environment (see below).
6. First deploy: push to `main` (or run `deploy.sh` manually with a digest-pinned image). Verify `GET https://<hostname>/api/health` through the edge.

Optional: install nightly backups from [`deploy/crontab.example`](deploy/crontab.example) and set `BACKUP_REMOTE`.

## Configuration

### Host env (`/opt/go-clipboard/.env`)

From [`deploy/env.example`](deploy/env.example). Secrets stay on the host — never in the repo or image.

| Variable            | Required      | Purpose                                                                                                                                                                                                       |
| ------------------- | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `PUBLIC_BASE_URL`   | yes           | Absolute HTTPS origin at the edge (no path/query/fragment). Server derives `Host` from this.                                                                                                                  |
| `POSTGRES_PASSWORD` | yes           | Compose Postgres password; interpolated into `DATABASE_URL`                                                                                                                                                   |
| `TRUSTED_PROXY`     | yes           | TCP peer IP of the edge as seen by the app. Same-LAN edge: the edge’s LAN IP. Edge arriving via the subnet router with Tailscale SNAT on: the router’s LAN IP. Honours `X-Forwarded-For` only from this peer. |
| `IMAGE`             | deploy arg    | Passed as the `deploy.sh` argument, not stored in `.env`: content digest (`registry/name@sha256:…`) — **never** `latest`. A full 40-char SHA tag is accepted only for manual rollback of a pre-digest deploy  |

`DATABASE_URL` is composed by `compose.yaml` / `deploy.sh` against the private `postgres` service. Do not publish Postgres to the host.

Keep `IMAGE` out of the host `.env`. `deploy.sh` validates its image argument first and sources the env file afterwards, so a stored `IMAGE=` line would silently replace the deploy target after the moving-tag check has already passed.

### GitHub Actions secrets

| Secret                                   | Purpose                                                                                                                                                                            |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `DOCKERHUB_USERNAME`                     | Registry namespace (`docker.io/<user>/go-clipboard`)                                                                                                                               |
| `DOCKERHUB_TOKEN`                        | Push credentials                                                                                                                                                                   |
| `DEPLOY_HOST`                            | SSH destination: Compose LXC LAN IP (`192.168.0.202`). The runner reaches it via the subnet route after joining Tailscale — not MagicDNS / `100.x` (the LXC is not on the tailnet) |
| `DEPLOY_USER`                            | SSH user                                                                                                                                                                           |
| `DEPLOY_SSH_KEY`                         | Private key (least-privilege deploy key). Ordinary SSH, not Tailscale SSH.                                                                                                         |
| `TS_OAUTH_CLIENT_ID` / `TS_OAUTH_SECRET` | Tailscale OAuth client (write `auth_keys`, tagged `tag:ci`). Required when the runner must join the tailnet to reach `:22`.                                                        |

Workflow env: `REGISTRY=docker.io`, `IMAGE_NAME=${{ secrets.DOCKERHUB_USERNAME }}/go-clipboard`. Deploys use the `production` GitHub Environment.

## CI over Tailscale

GitHub-hosted runners are on the public internet. They cannot SSH to a LAN-only `:22` unless the **runner joins the tailnet** and **accepts subnet routes**. That hop is separate from the edge → app `:8080` path (`TRUSTED_PROXY`). The Compose LXC does not run Tailscale; `tag:ci` is never applied to it.

`publish-and-deploy` joins the tailnet (ephemeral `tag:ci` node, `--accept-routes`) **before** Configure SSH / `scp` / `deploy.sh`.

### Tailnet ACL

Default tailnets allow `src`/`dst`/`ip: *`. That includes tagged devices, so adding a CI rule **on top of it does nothing** — replace that grant. Use **grants** (port in `ip`, not in `dst`).

Tag the **subnet router** `tag:homelab` (and the edge VPS too if it is on the tailnet). Do not tag the Compose LXC — it is not a tailnet node. `tag:homelab` is the router’s identity, not `192.168.0.202`; grants that should reach the LXC must use the advertised CIDR as `dst`. CI gets OpenSSH only — not `:8080`, not Postgres. The policy `"ssh"` section is Tailscale SSH; GitHub Actions uses ordinary SSH + `DEPLOY_SSH_KEY` and does not need a Tailscale SSH rule for `tag:ci`. Do not grant inbound access to `tag:ci`.

```json
{
	"tagOwners": {
		"tag:ci": ["autogroup:admin"],
		"tag:homelab": ["autogroup:admin"]
	},
	"grants": [
		{
			"src": ["autogroup:member"],
			"dst": ["autogroup:self", "tag:homelab", "192.168.0.0/24"],
			"ip": ["*"]
		},
		{
			"src": ["tag:homelab"],
			"dst": ["tag:homelab", "192.168.0.0/24"],
			"ip": ["*"]
		},
		{
			"src": ["tag:ci"],
			"dst": ["192.168.0.0/24"],
			"ip": ["tcp:22"]
		}
	],
	"tests": [
		{
			"src": "tag:ci",
			"proto": "tcp",
			"accept": ["192.168.0.202:22"],
			"deny": ["192.168.0.202:8080", "192.168.0.202:5432"]
		}
	]
}
```

Tighten CI `dst` to `192.168.0.202` if you only want that one host. Keep SSH off the public internet (LXC `:22` bound to LAN, or firewall it to the LAN / the subnet router).

### OAuth client

In the Tailscale admin console, create an [OAuth client](https://login.tailscale.com/admin/settings/oauth) with write **Devices Core** and **Auth Keys**, tags field `tag:ci` only. Do not use a personal reusable auth key. Store the client id and secret as `TS_OAUTH_CLIENT_ID` and `TS_OAUTH_SECRET`.

Set `DEPLOY_HOST` to the Compose LXC LAN IP (`192.168.0.202`). Approve the subnet router’s `192.168.0.0/24` route in the admin console if it is not already.

### Workflow step

[`publish-and-deploy`](.github/workflows/ci.yml) already includes this before **Configure SSH**:

```yaml
- name: Tailscale
  uses: tailscale/github-action@v4
  with:
    oauth-client-id: ${{ secrets.TS_OAUTH_CLIENT_ID }}
    oauth-secret: ${{ secrets.TS_OAUTH_SECRET }}
    tags: tag:ci
    args: --accept-routes
```

The action creates an **ephemeral** node; Linux runners do not accept subnet routes unless `--accept-routes` is set. The job SSHs with `DEPLOY_SSH_KEY` as already written; the node logs out when the job ends.

### Pin `known_hosts` for the LAN address

From a machine that can already reach the LXC (LAN or via the subnet route). Never `StrictHostKeyChecking=no`. Scan the **same string** as `DEPLOY_HOST`:

```bash
ssh-keyscan -t ed25519,rsa 192.168.0.202 >> deploy/known_hosts
```

Commit the scanned lines. A pin for MagicDNS / `100.x` will not match this destination.

### Alternative: ProxyJump through the public VPS

If the Caddy VPS is already on Tailscale (with subnet routes accepted) and has public SSH, the runner can SSH to the VPS and jump to `192.168.0.202`. Then the Tailscale Action is unnecessary, but the VPS must expose SSH and [`deploy/known_hosts`](deploy/known_hosts) must pin **both** hosts. Prefer joining the runner to the tailnet when CI is meant to SSH to the Compose host directly.

## CI / CD

On every commit / PR:

1. **Gate** — `make test`, `make lint`, `sqlc generate` clean-tree check
2. **Image** — build Dockerfile, run `deploy/smoke-test.sh`. On `main` the built image is saved as a workflow artifact, so the deploy job publishes the exact bits CI tested instead of rebuilding

On push to `main` only (after gate + image):

1. Load the image artifact and push tags: `<repo>:<full-git-sha>` and `<repo>:latest` (`latest` is convenience only)
2. Deploy target is the **content digest** (`<repo>@sha256:…`) read back from `RepoDigests`; the SHA tag is a pointer, not the identity. The `Guard deploy image digest` step refuses anything that is not a well-formed `sha256:` digest, and `deploy.sh` rejects moving tags again on the host
3. Join Tailscale (ephemeral `tag:ci` node, `--accept-routes`)
4. SCP `compose.yaml` + deploy/backup/restore scripts to `/opt/go-clipboard`
5. SSH: `deploy.sh <digest-image> [previous-digest-image]`

Deploys are serialized (`concurrency: production-deploy`). Workflow-level `cancel-in-progress` is off on `main` so a new push cannot abort an in-flight SSH deploy.

## Deploy flow (`deploy/deploy.sh`)

```text
flock → EXPECTED_PREVIOUS_SHA matches deployed-sha
  → up postgres → pull IMAGE → assert RepoDigest
  → migrate up
  → (optional) previous image healthy against migrated schema
  → candidate container on Compose network → poll GET /api/health
  → compose up -d app → write deployed-sha (sha256:<digest>)
```

- Migrations run as an explicit `docker run … <image> migrate up` step — **never** on container start.
- Candidate must return healthy health within `HEALTH_TIMEOUT_SEC` (default 90s) or the previous container keeps serving.
- `EXPECTED_PREVIOUS_SHA` (set by CI) refuses to clobber an unexpected live deployment.
- Brief outage during the published swap is expected (single instance).

Manual rollback / redeploy on the host:

```bash
cd /opt/go-clipboard
./deploy.sh docker.io/<user>/go-clipboard@sha256:<previous-digest>
```

## Rollback

1. Redeploy the previous digest with `deploy.sh` only — do not swap the app container by hand.
2. **Do not migrate down.** Rollback re-points the image; schema stays forward.
3. Each release runs a compatibility gate: after `migrate up`, the **previous** image must still serve healthy against the migrated schema, or the release is refused.

## Health contract

`GET /api/health` with `Host` matching the hostname from `PUBLIC_BASE_URL`:

| Outcome                             | Status  | Meaning                           |
| ----------------------------------- | ------- | --------------------------------- |
| DB reachable, no pending migrations | **200** | Deploy health gate passes         |
| Pending migrations or DB error      | **503** | Unhealthy; swap does not complete |

Health confirms DB reachability and migration state, not full query compatibility with a new schema — that is why the previous-image compatibility check exists.

Through the edge after deploy:

```bash
curl -sS "https://clip.example.com/api/health" | jq
```

Directly on the Compose host (bypasses the edge; still send the public `Host`):

```bash
curl -sS -H "Host: clip.example.com" http://127.0.0.1:8080/api/health | jq
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
- Proxy to the app at the Compose LXC LAN address on port `8080` (directly on LAN, or via Tailscale through the subnet router)
- Preserve/forward client identity so the app’s `TRUSTED_PROXY` can trust `X-Forwarded-For`

Misconfigured `TRUSTED_PROXY` breaks rate-limit client identity; wrong `Host` / `PUBLIC_BASE_URL` causes the app to reject requests.

## Constraints

- **One app replica** — do not scale the Compose `app` service.
- **No auto-migrate on start** — only `migrate up` via the image during deploy.
- **No** `latest` **deploys** — digest (`@sha256:…`) only; SHA tag is a convenience pointer.
- **Secrets outside image** — rotate by editing host `.env` / CI secrets; no rebuild required for secret-only changes.
- **Logs** — stdout/stderr only; collected by the container runtime.
- **One Compose owner** — CI `deploy.sh` owns `compose up`; Portainer does not recreate the stack from Git.
- **Out of scope here** — staging, blue/green, metrics/alerting, managed Postgres (easy upgrade later), shipping the public edge Caddyfile in-repo.
