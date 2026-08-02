## 1. Production container image

- [x] 1.1 Write a multi-stage `Dockerfile` at the repo root: a first stage runs `pnpm build` for the frontend, the Go build stage embeds that output via `//go:embed` and compiles the server, and a minimal runtime stage carries only the resulting binary
- [x] 1.2 Ensure the runtime stage contains no Go toolchain, Node, `pnpm`, or a separate frontend assets directory, and runs as a non-root user on an unprivileged port (e.g. 8080)
- [x] 1.3 Configure the image entrypoint so the same binary serves traffic by default and applies migrations when run with the `migrate up` argument
- [x] 1.4 Verify locally that the built image serves `GET /api/health` and the embedded frontend, and that `migrate up`/`migrate status` work from the same image against a reachable database
- [x] 1.5 Confirm serving the image while migrations are pending applies nothing and reports pending state via health

## 2. In-process frontend serving (Go)

- [x] 2.1 Add a Go package that embeds the built frontend (`//go:embed`) and expose it as an `fs.FS` the server can serve from
- [x] 2.2 Serve embedded assets and fall back to `index.html` for non-API paths that do not match an embedded file (SPA + `/<slug>` catch-all), registered so `/api/*` wins by matcher specificity regardless of registration order
- [x] 2.3 Serve plain HTTP in production; TLS terminates at the edge proxy, not in-process
- [x] 2.4 Enforce that requests whose `Host` does not match the hostname derived from `PUBLIC_BASE_URL` are rejected
- [x] 2.5 Add unit tests for asset serving and SPA fallback, and verify against the built image that assets serve directly, unknown routes fall back to `index.html`, and `/api/health` is handled unchanged

## 3. Deployment manifest (Compose behind edge)

- [x] 3.1 Write `compose.yaml` defining the app (SHA-tagged image) publishing plain HTTP on port 8080, and a Postgres service with a named volume — no TLS/ACME sidecars
- [x] 3.2 Wire `TRUSTED_PROXY` from the host env to the edge proxy IP
- [x] 3.3 Wire environment/secret injection from a store outside repo and image (env file on host or platform secrets), including `DATABASE_URL` and `PUBLIC_BASE_URL`
- [x] 3.4 Document in the manifest/comments that correctness depends on exactly one app instance (sweeper, migration locking, limiter state)
- [x] 3.5 Ensure logs go to stdout and are collected by the container runtime (no logging agent)

## 4. Runtime configuration

- [x] 4.1 Add `PUBLIC_BASE_URL` to the typed server configuration (read from env; documented default/validation) and to `.env.example`. Derive the public hostname from `PUBLIC_BASE_URL` as the single source of truth — validate that it is exactly one absolute HTTPS origin (no userinfo/path/query/fragment) and reject otherwise
- [x] 4.2 Set and document `TRUSTED_PROXY` as the edge proxy IP in production
- [x] 4.3 Confirm no secret is present in the repository or in image layers, and that changing a secret needs no image rebuild

## 5. CI/CD pipeline

- [x] 5.1 Add a CI workflow that runs the gate on every commit: `make test`, `make lint`
- [x] 5.2 Add a `sqlc generate` + clean-tree drift check that fails on any diff
- [x] 5.3 Build the container image on every commit (including non-deployed commits) so a broken `Dockerfile` fails CI
- [x] 5.4 On the default branch after the gate passes, publish the image to the public Docker Hub repository tagged with the full commit SHA, and also push a moving `latest` tag pointing at the same image (human convenience only — never a deploy target)
- [x] 5.5 Add the deploy step: over SSH, pull the SHA-tagged image on the host, run `migrate up` via the image, then swap the serving container. Pin and verify the production host's SSH key using a committed `known_hosts` entry (or equivalent trusted host-key configuration); keep SSH host-key checking enabled and never use `StrictHostKeyChecking=no`
- [x] 5.6 Gate the swap on `GET /api/health` reporting healthy (DB reachable, no pending migrations) before completing the deploy
- [x] 5.7 Reject deploys that reference a moving tag such as `latest`; only SHA tags are deployable
- [x] 5.8 Store production host, deploy SSH key, and registry credentials as CI secrets (least privilege)

## 6. Rollback and backups

- [x] 6.1 Document and verify rollback: redeploy the previous SHA tag via compose without rebuilding and without migrating down. Enforce backward compatibility as a release gate — after the new migration is applied, start the previous SHA image against the migrated schema and confirm it reports healthy via `GET /api/health`; fail the release if the previous image cannot run against the migrated schema, so a rollback-breaking migration is caught before it ships
- [x] 6.2 Add a nightly `pg_dump` of non-ephemeral data to off-box storage (excludes ephemeral `clips`)
- [x] 6.3 Verify a restore path from a `pg_dump` and confirm losing live clips is acceptable

## 7. Verification and docs

- [ ] 7.1 First end-to-end deploy: push to default branch, confirm the running service at the hostname over TLS (via the edge) is exactly that commit's image
- [x] 7.2 Confirm a half-landed deploy (image serving against un-migrated schema) is detectable via health
- [x] 7.3 Update the README with the deployment overview, required environment/secrets, and the rollback procedure
