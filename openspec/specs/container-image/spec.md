# container-image Specification

## Purpose

The production container image — a multi-stage build producing one immutable image per commit, identified by digest, with the frontend embedded in the server binary, an in-image migration runner, and CI smoke tests before publish.

## Requirements

### Requirement: The application ships as a production container image

The application SHALL be packaged as a container image built from a `Dockerfile` at the repository root. The image is a deployment artefact only; it MUST NOT be introduced into the development loop, and Postgres MUST NOT be containerised for development.

#### Scenario: Image is a deployment artefact, not a development tool

- **WHEN** a developer works on the project locally
- **THEN** the documented development loop remains `make dev`, `air`, and a locally installed Postgres, and running the container is not required for any development task

#### Scenario: The image runs the same server binary the project serves with

- **WHEN** the built image is run with the server's serve arguments
- **THEN** it starts the same Go server that development runs, reads its configuration from the environment, serves `GET /api/health`, and serves the frontend from assets embedded in the binary

### Requirement: The image is built multi-stage from source

The image SHALL be produced by a multi-stage build in which the frontend production build (`pnpm build`) runs before the Go compile so its output can be embedded into the server binary at compile time, and a minimal runtime stage carries only that binary. Build toolchains MUST NOT be present in the runtime stage, and the runtime stage MUST NOT carry the frontend assets as separate files, because they are compiled into the binary.

#### Scenario: Runtime stage excludes build toolchains

- **WHEN** the runtime image is inspected
- **THEN** it contains the server binary and does not contain the Go toolchain, Node, `pnpm`, or a separate directory of compiled frontend assets

#### Scenario: Frontend assets are embedded into the binary

- **WHEN** the image is built
- **THEN** the frontend build runs before the Go compile, its output is embedded into the server binary, and those embedded assets are the ones served in production

### Requirement: The image carries its own migration runner

The runtime image SHALL be able to apply and inspect migrations through the same binary it serves traffic with, invoked as a subcommand. There MUST NOT be a second tool or a second copy of the connection string for migrations.

#### Scenario: Migrations run from the same image with a different argument

- **WHEN** the image is run with the `migrate up` argument against a reachable database
- **THEN** pending migrations are applied by the same binary that otherwise serves traffic, using the same `DATABASE_URL`

#### Scenario: Serving does not apply migrations

- **WHEN** the image is run to serve traffic while migrations are pending
- **THEN** no migration is applied on startup and the pending state is reported by the health endpoint

### Requirement: One immutable image per commit, identified by digest

Each build SHALL produce one image whose immutable deployment identity is its content-addressable digest (`image@sha256:<digest>`), not the registry SHA tag, because a tag is a mutable pointer that can be repushed while a digest cannot. The full commit SHA is published as a convenience tag that maps a commit to its digest, and a moving tag such as `latest` MAY additionally be published as a human-convenience alias, but MUST NOT be used as a deploy target, because it makes "which commit is live" unanswerable. Deploys and rollbacks SHALL reference the image by digest, or — if a SHA tag is used — the registry MUST enforce immutable tags and the deploy MUST verify that the pulled image's digest matches the expected digest before serving it.

#### Scenario: Image identity is its digest, not its tag

- **WHEN** CI builds and publishes the image for a commit
- **THEN** the image is recorded and published by its content digest (`image@sha256:<digest>`), the commit SHA tag is published as a pointer to that digest, and any deploy resolves and verifies that digest rather than trusting the tag alone

#### Scenario: Image is tagged by commit SHA

- **WHEN** CI builds the image for a commit
- **THEN** the image is tagged with that commit's full SHA and pushed under that tag, and that tag resolves to the commit's immutable digest

#### Scenario: A moving convenience tag may also be published

- **WHEN** CI publishes an image from the default branch
- **THEN** it MAY also push a moving `latest` tag pointing at the same image, and doing so does not make `latest` a valid deploy target

#### Scenario: `latest` is refused as a deploy target

- **WHEN** a deploy is performed
- **THEN** it references an image by its commit SHA tag, and a deploy that names only a moving tag such as `latest` is rejected

#### Scenario: The tested image is the deployed image

- **WHEN** a commit's image is deployed
- **THEN** it is the same image CI built and tested, with nothing rebuilt from source on the host

### Requirement: The image is smoke-tested in CI before it is published

Before publishing an image, CI SHALL run a smoke test against the exact image it built — identified by digest — without rebuilding from source. The smoke test MUST validate the image entrypoint (the default serve command and the `migrate up` subcommand), that the embedded frontend is served, that the process runs as the non-root runtime user, that the migration runner applies migrations against a reachable database, and that `GET /api/health` answers. Publishing SHALL occur only after every smoke-test check passes.

#### Scenario: The built image passes smoke tests before publish

- **WHEN** CI has built an image for a commit
- **THEN** it runs the smoke test against that exact built image (by digest, without rebuilding from source), verifying the entrypoint, the embedded frontend, the non-root runtime user, the migration runner against a reachable database, and the `GET /api/health` endpoint, and only publishes the image after all of those checks pass

#### Scenario: A failing smoke-test check blocks publish

- **WHEN** any smoke-test check fails against the built image
- **THEN** the image is not published and the pipeline reports failure
