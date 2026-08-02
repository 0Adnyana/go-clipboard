## ADDED Requirements

### Requirement: Configuration is environment; secrets are outside repo and image

Production configuration SHALL be supplied entirely through environment variables, and secrets MUST NOT be committed to the repository or baked into the image. A secret baked into an image is a secret that needs a rebuild to rotate; the runtime MUST allow a secret to change without rebuilding the image.

#### Scenario: Secrets are supplied at runtime, not built in

- **WHEN** the production environment is inspected
- **THEN** secret values are provided from a store outside the repository and outside the image, and no secret value is present in the committed source or in the image layers

#### Scenario: A secret changes without rebuilding the image

- **WHEN** a secret value is changed in the runtime environment
- **THEN** the same SHA-tagged image continues to run with the new value and no image rebuild is required

### Requirement: The public base URL is configured and the edge hop is trusted

The runtime SHALL supply the public base URL of the deployment (the absolute URL later slices place inside links) as configuration whose value matches the deployment topology. Because an edge proxy terminates TLS in front of the Go server, the trusted forwarded hop MUST be configured to that edge's IP so that `X-Forwarded-For` from the trusted peer is honoured and the client identity used for rate limiting is the real client address.

#### Scenario: The public base URL is available to the application

- **WHEN** the server starts in production
- **THEN** it reads the configured public base URL from the environment, and it is the absolute origin at which the service answers at the edge

#### Scenario: The edge hop is trusted for client identity

- **WHEN** the application resolves a client IP for rate limiting in production
- **THEN** the trusted forwarded hop is the edge proxy IP, so `X-Forwarded-For` from that peer is honoured and a client-supplied forged value from an untrusted peer is not

### Requirement: Structured logs go to stdout

The runtime SHALL emit structured logs to stdout and rely on the container runtime to collect them. There MUST NOT be a log aggregation agent, shipping tier, or metrics/tracing dependency introduced by this capability.

#### Scenario: Logs are collected from stdout

- **WHEN** the container runs and emits log records
- **THEN** the records are written to stdout in structured form and are collected by the container runtime, with no additional logging agent required

### Requirement: Exactly one instance is an explicit constraint

The deployment SHALL run exactly one instance of the application. The single-instance assumptions carried from earlier slices — no migration advisory lock, one sweeper, in-process rate-limiter state — MUST be documented as depending on this constraint.

#### Scenario: A deploy is a single-instance swap

- **WHEN** a new image is deployed
- **THEN** the single running instance is replaced, in-process limiter state is discarded, and a brief outage during the swap is accepted rather than avoided by running a second instance

#### Scenario: The single-instance dependency is stated

- **WHEN** the deployment configuration or its documentation is read
- **THEN** it states that correctness of the sweeper, migration locking, and limiter state depends on the instance count being one

### Requirement: Deploy correctness is verified against a machine-readable health contract

`GET /api/health` SHALL expose a machine-readable contract the deploy evaluates. A response is **healthy** only when the database is reachable and the pending-migrations field is empty; the deploy MUST treat a healthy response as a success status (HTTP 200 with the pending-migrations field empty and no database error) and any other outcome — a non-empty pending-migrations field or a database error — as a **non-success** status (a non-2xx HTTP status) that fails the deploy health gate. The specific field and status the deploy evaluates (the HTTP status together with the empty pending-migrations field and the absence of a database error) SHALL be the same contract referenced by the pipeline and task specifications, so a half-landed deploy — a new image serving against an un-migrated schema — is both detectable and gating. The health-gated swap is **mandatory** (design decision D6): the deploy MUST hold the swap until the new instance returns a success status against this contract.

#### Scenario: A half-landed deploy is a non-success health status

- **WHEN** a new image is serving while its required migrations have not been applied
- **THEN** `GET /api/health` reports a non-empty pending-migrations field and returns a non-success status, distinguishing a half-landed deploy from a healthy one and failing the deploy health gate

#### Scenario: A database error is a non-success health status

- **WHEN** the database is unreachable or returns an error at health-check time
- **THEN** `GET /api/health` returns a non-success status, and the deploy health gate fails rather than treating the instance as healthy

#### Scenario: The mandatory health-gated swap holds until healthy

- **WHEN** the new instance does not return a success status against the health contract
- **THEN** the swap does not complete and the previous instance continues serving; the swap completes only once the new instance returns a success status (HTTP 200, empty pending-migrations field, no database error)

### Requirement: Backups cover only non-ephemeral data

Backups SHALL cover only data with no short lifetime ceiling — accounts and, from later slices, the audit log — and MUST NOT be required to preserve `clips`, which expire within 24 hours. A restore that loses every live clip is acceptable.

#### Scenario: Ephemeral clips are out of backup scope

- **WHEN** a backup is taken
- **THEN** it captures the non-ephemeral tables, and losing all live clips on restore is an accepted outcome rather than a failure
