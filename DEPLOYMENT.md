# Deployment

Operational runbook for running go-clipboard in production. Product intent for this slice lives in `[docs/slices/04-deployment.md](docs/slices/04-deployment.md)`; design decisions in `[openspec/changes/deployment/design.md](openspec/changes/deployment/design.md)`.

A deployment is **not** a public exposure — publishing the address remains a later gate. Exactly **one** app instance is required (in-process sweeper, migration locking, rate-limiter state).

## Topology

This is **edge TLS termination in front of a private origin**: a public reverse proxy terminates TLS; the app runs on a private Compose host (Docker LXC) that is **not** on Tailscale. A homelab **subnet router** advertises the LAN (`192.168.0.0/24`) so the edge and CI reach `192.168.0.202` through Tailscale. Releases are **push-based CD** — GitHub Actions joins the tailnet (with subnet routes), SSHs to that LAN address, and runs `[deploy/deploy.sh](deploy/deploy.sh)`. That is distinct from pull-based / GitOps (Portainer Git sync, watch `latest`).

```text
browser → edge Caddy (TLS / ACME)
       → Tailscale → subnet router (tag:homelab)
       → LAN 192.168.0.202:$APP_PORT  Compose LXC (Go, plain HTTP; no Tailscale)
            ├─ /api/*  → handlers → Postgres (Compose network only)
            └─ /*      → embedded SPA (go:embed)

GitHub runner (tag:ci, --accept-routes)
       → Tailscale → subnet router → 192.168.0.202:22
```

| Piece                      | Where                           | Notes                                                                             |
| -------------------------- | ------------------------------- | --------------------------------------------------------------------------------- |
| TLS / public hostname      | Public edge (Caddy on a VPS)    | Not in this repo; not terminated by Go                                            |
| App + Postgres             | Compose LXC at `192.168.0.202`  | No Tailscale on this host; image from Docker Hub; Postgres unpublished            |
| Subnet router              | Homelab node on the tailnet     | Tagged `tag:homelab`; advertises `192.168.0.0/24` (approved in the admin console) |
| Path edge → app `APP_PORT` | Tailscale → subnet → LAN        | TCP peer as seen by Go becomes `TRUSTED_PROXY`                                    |
| Path CI → host `:22`       | Tailscale → subnet → LAN        | Separate from the app path; see [CI over Tailscale](#ci-over-tailscale)           |
| Portainer                  | Optional UI on the Compose host | Watches the stack; does not deploy it                                             |

Do not keep a git checkout on the production host. The running artefact is the digest-pinned image; CI copies only `compose.yaml` and the deploy scripts into `/opt/go-clipboard`. Host secrets live in `/opt/go-clipboard/.env` (from `[deploy/env.example](deploy/env.example)`), never in the repo or image. The laptop `.env.example` is for `make dev` only — do not copy it onto the host.

## Setup checklist

Operator progress for this homelab. Details for each item are in [First-time host setup](#first-time-host-setup) and [CI over Tailscale](#ci-over-tailscale).

### Tailscale

- [x] `tagOwners` for `tag:ci` and `tag:homelab`
- [x] Tag **only** the subnet router `tag:homelab` (not the LXC; drop `tag:ci` from any always-on node — runners get it at job time)
- [x] Default `*` grant removed; `tag:ci` → `192.168.0.0/24` on `tcp:22`
- [x] Member and `tag:homelab` grants also include `192.168.0.0/24` (required once `*` is gone, or laptops/edge lose the LAN)
- [x] ACL `tests`: `tag:ci` accept `192.168.0.202:22`, deny the app port and `:5432`
- [x] OAuth client (write **Devices Core** + **Auth Keys**, tags field `tag:ci` only)
- [x] Subnet route `192.168.0.0/24` approved on the router in the admin console

### GitHub / repo

- [x] Actions secrets: `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`, `DEPLOY_HOST` (`192.168.0.202`), `DEPLOY_USER`, `DEPLOY_SSH_KEY`, `TS_OAUTH_CLIENT_ID`, `TS_OAUTH_SECRET`
- [x] GitHub Environment named `production` (the deploy job requires it)
- [x] `[deploy/known_hosts](deploy/known_hosts)` pins `192.168.0.202` (same string as `DEPLOY_HOST`)
- [x] Workflow Tailscale step passes no `args` (the action supplies `--accept-routes` on its own)

### Compose host (LXC)

- [x] Deploy key’s public half in `~/.ssh/authorized_keys` for `DEPLOY_USER`
- [x] `/opt/go-clipboard` exists and is **owned by `DEPLOY_USER`**
- [x] `/opt/go-clipboard/.env` written from `[deploy/env.example](deploy/env.example)`, readable by `DEPLOY_USER`
- [x] `DEPLOY_USER` in the `docker` group
- [x] Host can pull the image (`docker login` as `DEPLOY_USER` if the Docker Hub repository is private)
- [x] LXC `:22` not published on the public internet

### First deploy

- [x] Open a PR to `main` first — runs the gate and image build, deploys nothing
- [x] Merge to `main` so `publish-and-deploy` runs
- [ ] `GET https://<hostname>/api/health` through the edge returns 200

### Order of operations

Host bootstrap and the first push are independent, but this order gives the earliest feedback:

1. **Open a PR to `main`.** The workflow runs on `pull_request`, while `publish-and-deploy` is gated on `github.ref == 'refs/heads/main' && github.event_name == 'push'`. A PR therefore exercises the gate, the image build, and `smoke-test.sh` while deploying nothing.
2. **Bootstrap the host while CI runs** — see [First-time host setup](#first-time-host-setup).
3. **Merge.** `publish-and-deploy` then runs against a host that is already ready.

Pushing to `main` before the host is bootstrapped is safe, just not useful. `deploy.sh` sources `.env` before it takes the lock and before any `docker pull` or `compose up`, so a missing env file aborts the run without touching the host: nothing is left half-started and no `deployed-sha` is written.

Only a real deploy exercises the Tailscale join, the subnet route, and the `known_hosts` pin. A PR cannot.

## Usual setup: CI deploys, Portainer watches

1. Bootstrap `/opt/go-clipboard` and `.env` on the host (below).
2. Let GitHub Actions SSH in and run `deploy.sh` on push to `main`.
3. In Portainer **Stacks**, `go-clipboard` appears as an existing Compose project. Use Portainer for logs, inspect, and a one-off restart.

Do **not** recreate that stack from Git in Portainer. Do not let a Portainer Git stack and `deploy.sh` both run `compose up` on the same project. Do not scale `app`, auto-update from `latest`, or skip `migrate up` (Portainer “Deploy” is not a full release — migrations never run on container start).

## Artefacts

| Path                                                                              | Role                                                                                                     |
| --------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `[Dockerfile](Dockerfile)`                                                        | Multi-stage: `pnpm build` → Go embed → distroless non-root binary + migrations                           |
| `[compose.yaml](compose.yaml)`                                                    | App (`APP_PORT` → container `8080`) + optional Postgres 18 (`bundled-db` profile, named volume `pgdata`) |
| `[.github/workflows/ci.yml](.github/workflows/ci.yml)`                            | Gate on every commit; publish + SSH deploy on `main`                                                     |
| `[deploy/deploy.sh](deploy/deploy.sh)`                                            | Host-side: migrate → compatibility check → health-gated swap; rejects `latest`                           |
| `[deploy/smoke-test.sh](deploy/smoke-test.sh)`                                    | CI local smoke (migrate, non-root user, health, SPA, 503 while pending)                                  |
| `[deploy/backup.sh](deploy/backup.sh)` / `[deploy/restore.sh](deploy/restore.sh)` | Nightly dump (excludes `clips` data) and restore                                                         |
| `[deploy/env.example](deploy/env.example)`                                        | Template for the host `.env`                                                                             |
| `[deploy/known_hosts](deploy/known_hosts)`                                        | Pinned production SSH host keys (required before CI deploy)                                              |
| `[deploy/crontab.example](deploy/crontab.example)`                                | Nightly backup cron                                                                                      |

## First-time host setup

CI syncs `compose.yaml` and the deploy scripts to `/opt/go-clipboard` on **every** deploy. Do not clone this repository onto the host or copy those files by hand — the only file CI never writes is `.env`.

Three things must be true before the first deploy. All three are about identity: `deploy.sh` runs entirely as `DEPLOY_USER` over SSH, and contains no `sudo` anywhere.

| Requirement                                | Why                                                       |
| ------------------------------------------ | --------------------------------------------------------- |
| `/opt/go-clipboard` owned by `DEPLOY_USER` | CI `scp`s `compose.yaml` and the scripts into it          |
| `.env` readable by `DEPLOY_USER`           | `deploy.sh` sources it                                    |
| `DEPLOY_USER` in the `docker` group        | `deploy.sh` runs `docker pull` / `docker run` / `compose` |

Ownership is the easy thing to get wrong. A root-owned directory fails CI’s `scp`; a root-owned mode-600 `.env` fails `deploy.sh` at `source`. Neither surfaces until the first deploy.

### 1. Prepare the host

Provision a private host with Docker and the Compose v2 plugin. Ensure the public edge can reach `192.168.0.202:<APP_PORT>` (LAN, or Tailscale → subnet router), and point DNS at the edge rather than at this host.

Then, as a sudo-capable user:

```bash
DEPLOY_USER=<value of the DEPLOY_USER Actions secret>

sudo mkdir -p /opt/go-clipboard
sudo chown "$DEPLOY_USER:$DEPLOY_USER" /opt/go-clipboard
sudo usermod -aG docker "$DEPLOY_USER"   # harmless if already a member
```

The `chown` is unnecessary only when `DEPLOY_USER` is `root`.

### 2. Write the host env file

Create `/opt/go-clipboard/.env` from `[deploy/env.example](deploy/env.example)`. Writing it _as_ `DEPLOY_USER` sidesteps the ownership trap:

```bash
sudo -u "$DEPLOY_USER" tee /opt/go-clipboard/.env >/dev/null <<'EOF'
PUBLIC_BASE_URL=https://clip.example.com
COMPOSE_PROFILES=bundled-db
POSTGRES_PASSWORD=change-me
APP_PORT=8082
TRUSTED_PROXY=192.168.0.1
EOF
sudo chmod 600 /opt/go-clipboard/.env
```

Replace every placeholder; [Host env](#host-env-optgo-clipboardenv) explains what each one means. Two traps while editing:

- **This file is sourced as shell**, not parsed as key-value pairs. A `POSTGRES_PASSWORD` containing `$`, a backtick, or an unquoted `#` will expand, execute, or truncate. Use an alphanumeric password, or single-quote the value.
- **Never add an `IMAGE=` line** — see [Host env](#host-env-optgo-clipboardenv) for why a stored value defeats the moving-tag check.

Later deploys overwrite Compose and the scripts via `scp`; they never touch `.env`.

### 3. Verify as the deploy user

These three checks mirror what `deploy.sh` does, and catch every permission problem above before CI hits it:

```bash
sudo -u "$DEPLOY_USER" -H bash -lc 'docker info >/dev/null && echo "docker ok"'
sudo -u "$DEPLOY_USER" -H bash -lc 'touch /opt/go-clipboard/.probe && rm /opt/go-clipboard/.probe && echo "write ok"'
sudo -u "$DEPLOY_USER" -H bash -lc 'set -a; . /opt/go-clipboard/.env; set +a; echo "$PUBLIC_BASE_URL $TRUSTED_PROXY"'
```

If the Docker Hub repository is private, also run `docker login` once as `DEPLOY_USER`: `deploy.sh` pulls with the host’s own credentials, not CI’s. The health probe additionally pulls `curlimages/curl:8.5.0`, so the host needs outbound internet either way.

A fourth check covers the SSH identity CI uses, which is the one thing the three above cannot see. Compare the fingerprint the workflow’s **Configure SSH** step prints against the host’s `authorized_keys`, and confirm `sshd` will honour it:

```bash
sudo -u "$DEPLOY_USER" -H ssh-keygen -lf ~/.ssh/authorized_keys
sudo ls -ld ~"$DEPLOY_USER" ~"$DEPLOY_USER"/.ssh ~"$DEPLOY_USER"/.ssh/authorized_keys
sudo sshd -T | grep -Ei 'pubkeyauth|authorizedkeysfile|allowusers|allowgroups'
```

`sshd` enforces `StrictModes`: a group- or world-writable home directory makes it ignore `authorized_keys` entirely and refuse the key without logging anything on the client side. Want `.ssh` at `700` and `authorized_keys` at `600`.

### 4. Pin the SSH host key

Pin the host’s SSH host key into committed `[deploy/known_hosts](deploy/known_hosts)` (never `StrictHostKeyChecking=no`). Scan the same address CI will use — the LXC LAN IP (`192.168.0.202`), not a MagicDNS / `100.x` name (this host has none). See [CI over Tailscale](#ci-over-tailscale).

### 5. Configure GitHub

Add the Actions secrets and the `production` environment — see [GitHub Actions secrets](#github-actions-secrets).

### 6. First deploy

Merge to `main` (or run `deploy.sh` manually with a digest-pinned image), then confirm `GET https://<hostname>/api/health` through the edge returns 200.

Optional: install nightly backups from `[deploy/crontab.example](deploy/crontab.example)` and set `BACKUP_REMOTE`.

## Configuration

### Host env (`/opt/go-clipboard/.env`)

From `[deploy/env.example](deploy/env.example)`. Secrets stay on the host — never in the repo or image.

| Variable            | Required       | Purpose                                                                                                                                                                                                       |
| ------------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `PUBLIC_BASE_URL`   | yes            | Absolute HTTPS origin at the edge (no path/query/fragment). Server derives `Host` from this.                                                                                                                  |
| `POSTGRES_PASSWORD` | bundled DB     | Compose Postgres password; interpolated into `DATABASE_URL`. Not needed with an external database                                                                                                             |
| `COMPOSE_PROFILES`  | bundled DB     | `bundled-db` runs Postgres inside the stack. Unset it for an external database                                                                                                                                |
| `DATABASE_URL`      | external DB    | Full connection string. Setting it selects an [external database](#external-database) and disables the bundled Postgres                                                                                       |
| `TRUSTED_PROXY`     | yes            | TCP peer IP of the edge as seen by the app. Same-LAN edge: the edge’s LAN IP. Edge arriving via the subnet router with Tailscale SNAT on: the router’s LAN IP. Honours `X-Forwarded-For` only from this peer. |
| `APP_PORT`          | no (dflt 8080) | Host-side published port. The container always listens on `8080`; set this when `8080` is taken on the host, and point the edge at the same value                                                             |
| `IMAGE`             | deploy arg     | Passed as the `deploy.sh` argument, not stored in `.env`: content digest (`registry/name@sha256:…`) — **never** `latest`. A full 40-char SHA tag is accepted only for manual rollback of a pre-digest deploy  |

By default `DATABASE_URL` is composed by `compose.yaml` / `deploy.sh` against the private `postgres` service. Do not publish Postgres to the host.

`TRUSTED_PROXY` does not have to be right on the first deploy. Startup only requires a parseable IP, so a wrong-but-valid address still boots and still passes the health gate — the app simply ignores `X-Forwarded-For` and keys rate limits to the proxy’s own address instead of the real client. That is degraded, not broken. Correct it by editing `.env` and running `docker compose up -d app` on the host; no redeploy and no rebuild. An unparseable value, by contrast, fails startup outright and names the offending variable in the log.

### External database

The bundled Postgres is the `postgres` service behind the `bundled-db` Compose profile. To run against a managed or otherwise external database, edit the host `.env`: comment out `COMPOSE_PROFILES` and `POSTGRES_PASSWORD`, and set `DATABASE_URL` to the full connection string (managed providers normally require `sslmode=require`).

That single variable drives everything. `deploy.sh` skips `compose up -d postgres`, passes the external URL to the migrate, candidate, and rollback-check containers, and `backup.sh` / `restore.sh` switch from `compose exec postgres` to `pg_dump` / `psql` in a throwaway `postgres:18-alpine` container. Set `PG_CLIENT_IMAGE` if the external server's major version is newer than that image. The application itself needs no change — it treats `DATABASE_URL` as opaque.

Two things to know. `DATABASE_URL` must be set in the host `.env`, because `compose.yaml` lists it under `environment:`, which takes precedence over `env_file`; `deploy.sh` exports it so both agree. And switching an existing deployment does not migrate data or remove the old container — dump with `backup.sh` first, restore into the new database, then `docker compose down` the stale `postgres` service. The `pgdata` volume is left in place.

Keep `IMAGE` out of the host `.env`. `deploy.sh` validates its image argument first and sources the env file afterwards, so a stored `IMAGE=` line would silently replace the deploy target after the moving-tag check has already passed.

On a host that has **never** run this stack, create the Compose network before the first external-database deploy — `docker network create go-clipboard_internal`. `deploy.sh` skips `compose up -d postgres` whenever `DATABASE_URL` is set, and that skipped step is what would otherwise create the network its `migrate` and candidate containers attach to. Hosts that previously ran the bundled profile already have it.

### GitHub Actions secrets

| Secret                                   | Purpose                                                                                                                                                                            |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `DOCKERHUB_USERNAME`                     | Registry namespace (`docker.io/<user>/go-clipboard`)                                                                                                                               |
| `DOCKERHUB_TOKEN`                        | Push credentials                                                                                                                                                                   |
| `DEPLOY_HOST`                            | SSH destination: Compose LXC LAN IP (`192.168.0.202`). The runner reaches it via the subnet route after joining Tailscale — not MagicDNS / `100.x` (the LXC is not on the tailnet) |
| `DEPLOY_USER`                            | SSH user                                                                                                                                                                           |
| `DEPLOY_SSH_KEY`                         | Private key (least-privilege deploy key), unencrypted, whole file including both `-----BEGIN/END-----` lines. Ordinary SSH, not Tailscale SSH.                                     |
| `TS_OAUTH_CLIENT_ID` / `TS_OAUTH_SECRET` | Tailscale OAuth client (write `auth_keys`, tagged `tag:ci`). Required when the runner must join the tailnet to reach `:22`.                                                        |

Workflow env: `REGISTRY=docker.io`, `IMAGE_NAME=${{ secrets.DOCKERHUB_USERNAME }}/go-clipboard`. Deploys use the `production` GitHub Environment.

**Configure SSH** rejects a `DEPLOY_SSH_KEY` that is passphrase-protected, public-half-only, or line-folded, and prints the fingerprint of the key CI will offer. **Verify SSH auth** then proves the key before any `scp` runs, so a rejected key fails on its own named step instead of surfacing as a confusing error inside a later step. The SSH client config sets `PreferredAuthentications publickey` and `BatchMode yes`: a runner has no TTY, so password fallback can only fail, and letting it try turns a plain `Permission denied (publickey)` into two misleading `Permission denied, please try again.` lines. A failure here is server-side — the key is not in `authorized_keys` for `DEPLOY_USER`, `StrictModes` is rejecting home directory permissions, or `sshd` disallows the user. See [Verify as the deploy user](#3-verify-as-the-deploy-user).

## CI over Tailscale

GitHub-hosted runners are on the public internet. They cannot SSH to a LAN-only `:22` unless the **runner joins the tailnet** and **accepts subnet routes**. That hop is separate from the edge → app `APP_PORT` path (`TRUSTED_PROXY`). The Compose LXC does not run Tailscale; `tag:ci` is never applied to it.

`publish-and-deploy` joins the tailnet (ephemeral `tag:ci` node, `--accept-routes`) **before** Configure SSH / `scp` / `deploy.sh`.

### Tailnet ACL

Default tailnets allow `src`/`dst`/`ip: *`. That includes tagged devices, so adding a CI rule **on top of it does nothing** — replace that grant. Use **grants** (port in `ip`, not in `dst`).

Tag the **subnet router** `tag:homelab` (and the edge VPS too if it is on the tailnet). Do not tag the Compose LXC — it is not a tailnet node. `tag:homelab` is the router’s identity, not `192.168.0.202`; grants that should reach the LXC must use the advertised CIDR as `dst`. CI gets OpenSSH only — not the app port, not Postgres. The policy `"ssh"` section is Tailscale SSH; GitHub Actions uses ordinary SSH + `DEPLOY_SSH_KEY` and does not need a Tailscale SSH rule for `tag:ci`. Do not grant inbound access to `tag:ci`.

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
			"deny": ["192.168.0.202:8080", "192.168.0.202:8082", "192.168.0.202:5432"]
		}
	]
}
```

Tighten CI `dst` to `192.168.0.202` if you only want that one host. Keep SSH off the public internet (LXC `:22` bound to LAN, or firewall it to the LAN / the subnet router).

### OAuth client

In the Tailscale admin console, create an [OAuth client](https://login.tailscale.com/admin/settings/oauth) with write **Devices Core** and **Auth Keys**, tags field `tag:ci` only. Do not use a personal reusable auth key. Store the client id and secret as `TS_OAUTH_CLIENT_ID` and `TS_OAUTH_SECRET`.

Set `DEPLOY_HOST` to the Compose LXC LAN IP (`192.168.0.202`). Approve the subnet router’s `192.168.0.0/24` route in the admin console if it is not already.

### Workflow step

`[publish-and-deploy](.github/workflows/ci.yml)` already includes this before **Configure SSH**:

```yaml
- name: Tailscale
  uses: tailscale/github-action@v4
  with:
    oauth-client-id: ${{ secrets.TS_OAUTH_CLIENT_ID }}
    oauth-secret: ${{ secrets.TS_OAUTH_SECRET }}
    tags: tag:ci
```

The action creates an **ephemeral** node. Linux runners do not accept subnet routes by default, but `tailscale/github-action@v4` already passes `--accept-routes` itself — do not add it through `args`, or `tailscale up` fails with `invalid boolean flag accept-routes: flag provided multiple times`. The job SSHs with `DEPLOY_SSH_KEY` as already written; the node logs out when the job ends.

### Pin `known_hosts` for the LAN address

From a machine that can already reach the LXC (LAN or via the subnet route). Never `StrictHostKeyChecking=no`. Scan the **same string** as `DEPLOY_HOST`:

```bash
ssh-keyscan -t ed25519,rsa 192.168.0.202 >> deploy/known_hosts
```

Commit the scanned lines. A pin for MagicDNS / `100.x` will not match this destination.

### Alternative: ProxyJump through the public VPS

If the Caddy VPS is already on Tailscale (with subnet routes accepted) and has public SSH, the runner can SSH to the VPS and jump to `192.168.0.202`. Then the Tailscale Action is unnecessary, but the VPS must expose SSH and `[deploy/known_hosts](deploy/known_hosts)` must pin **both** hosts. Prefer joining the runner to the tailnet when CI is meant to SSH to the Compose host directly.

## CI / CD

On every commit / PR:

1. **Gate** — `make test`, `make lint`, `sqlc generate` clean-tree check
2. **Image** — build Dockerfile, run `deploy/smoke-test.sh`. On `main` the built image is saved as a workflow artifact, so the deploy job publishes the exact bits CI tested instead of rebuilding

On push to `main` only (after gate + image):

1. Load the image artifact and push tags: `<repo>:<full-git-sha>` and `<repo>:latest` (`latest` is convenience only)
2. Deploy target is the **content digest** (`<repo>@sha256:…`) read back from `RepoDigests`; the SHA tag is a pointer, not the identity. The `Guard deploy image digest` step refuses anything that is not a well-formed `sha256:` digest, and `deploy.sh` rejects moving tags again on the host
3. Join Tailscale (ephemeral `tag:ci` node, `--accept-routes`)
4. Configure SSH, then verify publickey auth before anything is copied
5. SCP `compose.yaml` + deploy/backup/restore scripts to `/opt/go-clipboard`
6. SSH: `deploy.sh <digest-image> [previous-digest-image]`

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

Directly on the Compose host (bypasses the edge; still send the public `Host`). Use `APP_PORT`, not the container’s `8080`:

```bash
curl -sS -H "Host: clip.example.com" http://127.0.0.1:8082/api/health | jq
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
- Proxy to the app at the Compose LXC LAN address on the host’s `APP_PORT` (directly on LAN, or via Tailscale through the subnet router)
- Preserve/forward client identity so the app’s `TRUSTED_PROXY` can trust `X-Forwarded-For`

Misconfigured `TRUSTED_PROXY` breaks rate-limit client identity; wrong `Host` / `PUBLIC_BASE_URL` causes the app to reject requests.

## Constraints

- **One app replica** — do not scale the Compose `app` service.
- **No auto-migrate on start** — only `migrate up` via the image during deploy.
- **No** `latest` **deploys** — digest (`@sha256:…`) only; SHA tag is a convenience pointer.
- **Secrets outside image** — rotate by editing host `.env` / CI secrets; no rebuild required for secret-only changes.
- **Logs** — stdout/stderr only; collected by the container runtime.
- **One Compose owner** — CI `deploy.sh` owns `compose up`; Portainer does not recreate the stack from Git.
- **Out of scope here** — staging, blue/green, metrics/alerting, shipping the public edge Caddyfile in-repo.
