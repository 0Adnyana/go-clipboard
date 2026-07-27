# persistence Specification

## Purpose

Database access — pooled connections, startup connectivity verification, and a generated type-safe query layer exposed behind an interface for testing.

## Requirements

### Requirement: A single pooled database connection

The application SHALL create exactly one `pgxpool` connection pool from `DATABASE_URL` at startup and share it across all database consumers. The application MUST NOT open a second pool or a second connection string for any purpose.

#### Scenario: Pool is created from the connection string

- **WHEN** the process starts with a valid `DATABASE_URL`
- **THEN** one connection pool is created from it and made available to the HTTP handlers

#### Scenario: Ownership of the pool is explicit

- **WHEN** the process shuts down
- **THEN** the pool is closed by the same wiring code that created it, and no other component closes it

### Requirement: Startup connectivity verification

At startup the application SHALL verify that the database is reachable and log the outcome. A failed verification MUST NOT prevent the HTTP server from starting, so that the health endpoint remains available to report the problem.

#### Scenario: Reachable database is confirmed at startup

- **WHEN** the process starts and Postgres is accepting connections
- **THEN** an informational record is logged confirming database connectivity

#### Scenario: Unreachable database does not stop the server

- **WHEN** the process starts and Postgres is not running
- **THEN** the failure is logged with the underlying error and the HTTP server still starts and serves `GET /api/health`

### Requirement: Database reachability probe

The persistence layer SHALL expose an operation that checks database reachability within a bounded timeout and reports both the result and the observed round-trip latency, without leaving the caller blocked indefinitely.

#### Scenario: Probe reports latency when the database answers

- **WHEN** the probe runs against a reachable database
- **THEN** it reports success along with the measured latency

#### Scenario: Probe fails fast when the database does not answer

- **WHEN** the probe runs against an unreachable database
- **THEN** it returns an error within the bounded timeout rather than hanging

### Requirement: Generated type-safe query layer

All SQL executed by the application SHALL come from `sqlc`-generated code produced from `db/queries` against the schema in `db/migrations`, targeting the `pgx/v5` driver. Application code MUST NOT contain hand-written SQL strings, and generated files MUST NOT be edited by hand.

#### Scenario: Generated code is regenerated from source SQL

- **WHEN** a query file in `db/queries` changes and code generation is run
- **THEN** the generated Go code is updated to match, and the change is visible to the compiler

#### Scenario: Schema and generated code cannot drift

- **WHEN** code generation runs
- **THEN** it reads the schema from the migrations directory rather than from a separate schema file, so there is one source of truth for both runtime and codegen

### Requirement: Query layer is consumable behind an interface

Code generation SHALL emit an interface describing the generated query set, and consumers of the query layer SHALL depend on that interface rather than on the concrete generated struct, so that they can be unit-tested with a fake and no database.

#### Scenario: Consumer depends on the interface

- **WHEN** a handler or service needs database access
- **THEN** it accepts the generated query interface as a dependency

#### Scenario: Consumer is testable without a database

- **WHEN** a unit test exercises a consumer of the query layer
- **THEN** it can substitute an in-memory implementation of the interface and run without Postgres

### Requirement: A `database/sql` view of the pool for tooling

Components that require a `database/sql` handle SHALL obtain one adapted from the existing `pgxpool` rather than by opening a new connection. Closing that adapted handle does not close the pool, so the pool MUST still be closed explicitly by its owner.

#### Scenario: Tooling shares the application pool

- **WHEN** the migration runner needs a `database/sql` handle
- **THEN** it is given one derived from the existing pool, and no additional pool is created

#### Scenario: Pool outlives the adapted handle

- **WHEN** the adapted `database/sql` handle is closed
- **THEN** the underlying pool remains usable until its owner closes it

### Requirement: The skeleton exercises the generated query path end to end

The query set SHALL include at least one query that requires no domain tables, and the application SHALL execute it through the generated code so the full path from HTTP handler to Postgres and back is proven without inventing schema that a later change would have to delete.

#### Scenario: Generated query returns data from Postgres

- **WHEN** the endpoint that uses the generated query is requested
- **THEN** the response reflects a value that originated from the database via the generated code

#### Scenario: The skeleton query does not depend on domain tables

- **WHEN** the query runs against a database with only the baseline migration applied
- **THEN** it succeeds without requiring any application table to exist

### Requirement: Clips are stored through the generated query layer

Clip persistence SHALL be expressed as SQL in `db/queries` and consumed only through the `sqlc`-generated `Querier` interface. Handlers and services MUST NOT embed ad-hoc SQL strings for create, reclaim, or read.

#### Scenario: Create and read go through generated methods

- **WHEN** a clip is created or a live clip is read
- **THEN** the operation executes via generated `Querier` methods backed by checked-in query files

#### Scenario: Service logic is unit-testable with a fake Querier

- **WHEN** unit tests exercise claim, collision, and read behaviour
- **THEN** they substitute an in-memory `Querier` and run without Postgres

### Requirement: Live lookup and atomic claim are expressible in SQL

The query set SHALL support reading a live clip by exact slug and claiming a slug only when no live row holds it, including reclaim of an expired row, without a check-then-insert race in application code.

#### Scenario: Live read ignores expired rows

- **WHEN** a read query runs for a slug whose row is expired
- **THEN** it returns no live clip

#### Scenario: Claim fails while a live row exists

- **WHEN** a claim query runs for a slug with `expires_at` still in the future
- **THEN** no overwrite occurs and the caller can detect the conflict
