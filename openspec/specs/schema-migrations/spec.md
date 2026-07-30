# schema-migrations Specification

## Purpose

Explicit, developer-invoked schema management — migrations applied only via the `migrate` subcommand, never implicitly, with pending migrations detectable without applying them.
## Requirements
### Requirement: Migrations apply only when explicitly invoked

Schema changes SHALL be applied only by an explicit `migrate up` invocation of the server binary. Starting the HTTP server MUST NOT apply migrations under any configuration, and there MUST be no flag or environment variable that makes startup apply them.

#### Scenario: Applying pending migrations

- **WHEN** `migrate up` is invoked with pending migration files present
- **THEN** the pending migrations are applied in order, each application is logged, and the process exits with a zero status

#### Scenario: Server startup leaves the schema untouched

- **WHEN** the HTTP server is started while migrations are pending
- **THEN** the schema is unchanged, the server serves requests, and the pending state is reported by the health endpoint

#### Scenario: Applying migrations when nothing is pending

- **WHEN** `migrate up` is invoked and the schema is already current
- **THEN** no migration is applied and the process exits with a zero status

### Requirement: Migration status is inspectable

The binary SHALL provide a `migrate status` command that prints each known migration with whether it has been applied, and the current schema version, without modifying the database.

#### Scenario: Status lists applied and pending migrations

- **WHEN** `migrate status` is invoked
- **THEN** each migration is listed with its applied state and the current version is shown

#### Scenario: Status does not change the schema

- **WHEN** `migrate status` is invoked while migrations are pending
- **THEN** the pending migrations remain unapplied afterwards

### Requirement: Pending migrations are detectable without applying them

The migration layer SHALL expose a check that reports whether unapplied migrations exist and what the current version is, usable from a request handler. That check MUST NOT apply migrations and MUST NOT take a database lock that could block a concurrent migration run.

#### Scenario: Pending state is reported to the health endpoint

- **WHEN** unapplied migration files exist and the health endpoint is requested
- **THEN** the response reports migrations as pending along with the current version, and no migration is applied

#### Scenario: Detection tolerates an uninitialised database

- **WHEN** the check runs against a database that has never had migrations applied
- **THEN** it reports migrations as pending rather than returning an error that breaks the health response

#### Scenario: A failed check is distinguishable from an uninitialised database

- **WHEN** the check cannot reach the database at all
- **THEN** it reports the failure to its caller rather than a zero-valued status, so "never migrated" and "could not look" are never conflated

### Requirement: Migrations are reversible by command

The binary SHALL provide a `migrate down` command that rolls back the most recently applied migration using its down statements.

#### Scenario: Rolling back the latest migration

- **WHEN** `migrate down` is invoked with at least one migration applied
- **THEN** the most recent migration is rolled back, the action is logged, and the current version decreases accordingly

### Requirement: Migrations are read from a configurable directory on disk

Migration files SHALL be read from the filesystem at run time through a configurable directory path defaulting to `db/migrations`, rather than being compiled into the binary, so a new migration is visible without a rebuild.

#### Scenario: A new migration file is picked up without rebuilding

- **WHEN** a migration file is added to the migrations directory and `migrate status` is invoked with the existing binary
- **THEN** the new migration appears in the output as pending

#### Scenario: Directory is resolved relative to the working directory

- **WHEN** a migration command is invoked from the repository root with the default configuration
- **THEN** the migrations in `db/migrations` are found

#### Scenario: Missing migrations directory is reported clearly

- **WHEN** a migration command is invoked with a migrations directory that does not exist
- **THEN** the process reports the path it tried to read and exits with a non-zero status

### Requirement: Migration filenames are timestamp-prefixed

Every migration file SHALL be named with a numeric timestamp prefix, so that lexicographic ordering and chronological ordering are identical. Hand-numbered or zero-padded sequential prefixes MUST NOT be used, because the code generator reads the directory lexicographically while the migration runner orders numerically.

#### Scenario: Created migration carries a timestamp prefix

- **WHEN** a new migration is created through the documented creation command
- **THEN** its filename begins with a timestamp and it sorts after all existing migrations both lexicographically and numerically

#### Scenario: Generator and runner agree on order

- **WHEN** code generation and `migrate up` both read the migrations directory
- **THEN** they process the files in the same order

### Requirement: A baseline migration exercises the runner

The migrations directory SHALL contain at least one migration that is domain-neutral and idempotent in effect, so the runner has a real version to apply and the version table is provably created. It MUST NOT introduce schema that a later domain change would have to delete.

#### Scenario: Fresh database reaches a known version

- **WHEN** `migrate up` is run against a newly created empty database
- **THEN** the version tracking table is created, the baseline migration is applied, and the reported current version is non-zero

#### Scenario: Baseline migration is reversible

- **WHEN** the baseline migration is rolled back and applied again
- **THEN** both operations succeed without error

### Requirement: Clips table migration is explicit and reversible

A goose migration SHALL introduce the clips storage needed for paste-and-read, including a uniqueness constraint on the exact slug string. The migration MUST be applied only via the existing `migrate` commands, MUST include a real down migration, and MUST NOT run on server startup.

#### Scenario: Migrate up creates clips storage

- **WHEN** `migrate up` is run with the clips migration pending
- **THEN** the clips table and its uniqueness constraints exist and health reports migrations not pending

#### Scenario: Migrate down removes clips storage

- **WHEN** `migrate down` rolls back the clips migration
- **THEN** the clips table is removed and a subsequent up can reapply it

### Requirement: Clips table autovacuum is tuned by a reversible migration

A goose migration SHALL tune autovacuum for the clips table so that its high insert-and-delete churn does not accumulate dead tuples faster than they are reclaimed. The migration MUST be applied only via the existing `migrate` commands, MUST include a real down migration that restores the table to default autovacuum behaviour, and MUST NOT run on server startup. It MUST NOT add or drop columns, because creation time and expiry already carry everything TTL requires.

#### Scenario: Migrate up applies autovacuum settings

- **WHEN** `migrate up` is run with the autovacuum-tuning migration pending
- **THEN** the clips table carries the tuned autovacuum storage parameters and health reports migrations not pending

#### Scenario: Migrate down restores defaults

- **WHEN** `migrate down` rolls back the autovacuum-tuning migration
- **THEN** the tuned parameters are removed so the table reverts to default autovacuum behaviour, and a subsequent up can reapply them

#### Scenario: Autovacuum migration changes no columns

- **WHEN** the autovacuum-tuning migration is applied
- **THEN** the clips table's columns are unchanged and existing rows remain readable

