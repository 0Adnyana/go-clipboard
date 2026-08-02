## ADDED Requirements

### Requirement: The pipeline refuses before it ships

On every commit, the CI pipeline SHALL run the project's gate — `make test` and `make lint` — before any image is published or deployed. A failure in any gating job MUST stop the pipeline before publish and deploy.

#### Scenario: A failing test blocks the pipeline

- **WHEN** a commit is pushed for which `make test` fails
- **THEN** the pipeline reports failure and does not publish or deploy an image

#### Scenario: A lint failure blocks the pipeline

- **WHEN** a commit is pushed for which `make lint` fails
- **THEN** the pipeline reports failure and does not publish or deploy an image

### Requirement: Generated code cannot drift from its source

The pipeline SHALL run `sqlc generate` and then verify the working tree is clean, failing if generated code differs from what the migrations and queries produce, so the guarantee is enforced by CI rather than by the developer remembering.

#### Scenario: Drift between generated code and its source fails CI

- **WHEN** `sqlc generate` produces output that differs from the committed generated code
- **THEN** the clean-tree check fails and the pipeline reports failure

#### Scenario: Matching generated code passes the drift check

- **WHEN** `sqlc generate` produces no change to the working tree
- **THEN** the drift check passes

### Requirement: The image is built on every commit

The pipeline SHALL build the container image on every commit, including commits that are not deployed, so a broken `Dockerfile` is discovered by CI rather than at deploy time.

#### Scenario: A broken Dockerfile is caught on any commit

- **WHEN** a commit whose `Dockerfile` fails to build is pushed
- **THEN** the pipeline reports failure on that commit even if it would not be deployed

### Requirement: Merges to the default branch publish and deploy

When the gate passes on the default branch, the pipeline SHALL publish the commit's SHA-tagged image to the registry and deploy it, applying migrations as an explicit step before the serving image is swapped. Default-branch deploy runs MUST be serialized by a shared CI concurrency group so that only one deploy runs at a time, and the host-side migration, rollback, and serving-image swap operations MUST be protected by a lock or a compare-and-swap check so that a run started from an older commit cannot replace a newer deployment or run migration and rollback actions out of order.

#### Scenario: A passing commit on the default branch is deployed

- **WHEN** the gate passes for a commit on the default branch
- **THEN** its SHA-tagged image is published to the registry, migrations are applied via that image, and the running service is swapped to that image

#### Scenario: Concurrent default-branch deploys are serialized

- **WHEN** two default-branch commits reach the deploy stage close together
- **THEN** a shared CI concurrency group forces the runs to execute one at a time rather than concurrently

#### Scenario: An older run cannot overwrite a newer deployment

- **WHEN** a deploy run started from an older commit reaches the host-side migration, swap, or rollback step after a newer commit has already been deployed
- **THEN** a lock or compare-and-swap check detects that the live deployment is newer and refuses the older run's migration, swap, and rollback actions rather than replacing the newer deployment or running out of order

#### Scenario: Migrations are a deploy step, not a startup step

- **WHEN** a deploy is performed
- **THEN** migrations are applied by an explicit pipeline step using the image's migration runner, and no migration is applied by the serving container on start

#### Scenario: Commits off the default branch are gated but not deployed

- **WHEN** the gate runs for a commit that is not on the default branch
- **THEN** the gate and image build run, and no publish or deploy occurs

### Requirement: Rollback re-points to the previous image

Rollback SHALL be performed by pointing the running service back at a previously published SHA-tagged image, without a rebuild. This is valid only while each migration is backward-compatible with the image before it; that compatibility is a discipline the pipeline does not enforce.

#### Scenario: Reverting to the previous image restores the prior version

- **WHEN** a deploy is found to be bad and the previous SHA-tagged image is redeployed
- **THEN** the running service returns to that previous image without rebuilding from source

#### Scenario: Rollback does not migrate down

- **WHEN** a rollback re-points the service to the previous image
- **THEN** it changes only which image runs and does not roll the database schema backward
