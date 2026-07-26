# developer-workflow Specification

## Purpose

A reproducible local environment — a fresh clone reaches a running full stack in a documented, minimal number of commands, with hot reload on both sides, a supervised set of processes behind one URL, and a single command for the schema-to-generated-code loop.

## Requirements

### Requirement: A Makefile is the single entry point for development tasks

Every routine development action SHALL be available as a Make target, so a developer does not need to remember the underlying tool invocations. The documented targets SHALL cover starting the stack, migrating, generating code, creating and resetting the database, testing, and linting.

#### Scenario: Available tasks are discoverable

- **WHEN** the developer inspects the Makefile or runs its help target
- **THEN** each supported development task is listed with a one-line description

#### Scenario: Underlying tools are not invoked directly in documentation

- **WHEN** the README describes a routine development action
- **THEN** it names the Make target rather than the raw tool command

### Requirement: A fresh clone reaches a running stack in documented steps

The README SHALL document the prerequisites — a locally installed Postgres with its client tools, a Node package manager, the Go toolchain, and the proxy binary — and the ordered, minimal sequence of commands that takes a clean clone to a working stack, including the version of Postgres the project is developed against.

#### Scenario: Documented sequence works on a clean clone

- **WHEN** a developer follows the README setup steps in order on a machine with the prerequisites installed
- **THEN** the stack is running and the status page reports the database as reachable with no pending migrations

#### Scenario: Prerequisites are stated with install commands

- **WHEN** the developer reads the prerequisites section
- **THEN** each prerequisite is listed with the command to install it

### Requirement: One command starts the whole stack behind one URL

A single Make target SHALL start the proxy, the Go server with hot reload, and the frontend dev server concurrently, and SHALL print the proxy URL as the address to open. It MUST NOT attempt to start or manage Postgres.

#### Scenario: All three processes start together

- **WHEN** the developer runs the development target
- **THEN** the proxy, the Go server, and the frontend dev server are all running and the application is reachable at the printed proxy URL

#### Scenario: Only the supported URL is advertised

- **WHEN** the development target prints its startup output
- **THEN** the proxy URL is the only entry point it tells the developer to open

#### Scenario: The database is assumed to be running

- **WHEN** the development target runs
- **THEN** it does not start, stop, or provision Postgres

### Requirement: Stopping development cleans up every child process

The development target SHALL trap interrupt and termination signals and terminate all processes it started, so that none of the three ports remains bound after it exits.

#### Scenario: Interrupt releases all ports

- **WHEN** the developer interrupts the development target
- **THEN** all three child processes exit and the proxy, API, and dev server ports are all free

#### Scenario: The stack restarts immediately afterwards

- **WHEN** the development target is run again right after being interrupted
- **THEN** it starts successfully without an address-already-in-use failure

### Requirement: Missing prerequisites fail with an actionable message

Before starting the stack, the development target SHALL check that the required external tools are on the `PATH` and SHALL fail with the name of the missing tool and the command to install it, rather than with a bare command-not-found error.

#### Scenario: Proxy binary is missing

- **WHEN** the development target runs on a machine without the proxy binary installed
- **THEN** it stops with a message naming the tool and giving its install command

#### Scenario: Postgres is not running

- **WHEN** the stack is started while Postgres is not accepting connections
- **THEN** the server logs the connection failure and the status page reports the database as unreachable, so the diagnosis uses the same path as any other database problem

### Requirement: Environment configuration lives in one file

The Makefile SHALL load and export variables from a local `.env` file so that the Go server, the proxy, and the dev server all read the same values. A committed `.env.example` SHALL document every variable — the database URL and the three ports — and MUST show a database URL in the form that works with a Homebrew-installed Postgres, with a comment explaining why it differs from the conventional form.

#### Scenario: Copying the example produces a working configuration

- **WHEN** the developer copies `.env.example` to `.env` and substitutes their own username
- **THEN** the stack starts and connects to the database without further edits

#### Scenario: A port is defined exactly once

- **WHEN** a port value is changed in `.env`
- **THEN** the proxy, the Go server, and the dev server all pick up the new value without any other file being edited

#### Scenario: The connection string shape is explained

- **WHEN** the developer reads `.env.example`
- **THEN** it shows an account-named superuser with no password and a comment stating that the conventional form fails against a Homebrew Postgres

#### Scenario: The real environment file is not committed

- **WHEN** the repository status is checked after creating a local `.env`
- **THEN** the file is ignored by version control

### Requirement: Database lifecycle is managed by dedicated targets

Make targets SHALL create the development database and reset it. The reset target MUST force-disconnect existing clients so it succeeds while the server is running, and MUST recreate and re-migrate the database afterwards.

#### Scenario: Creating the database

- **WHEN** the create target runs against a Postgres instance without the development database
- **THEN** the database is created and the stack can connect to it

#### Scenario: Resetting while the server holds connections

- **WHEN** the reset target runs while the Go server is running and holding pool connections
- **THEN** the drop succeeds anyway, and the database is recreated and migrated to the current version

#### Scenario: Creating an existing database is not destructive

- **WHEN** the create target runs and the database already exists
- **THEN** existing data is not dropped

### Requirement: The schema-to-generated-code loop is one command

A single Make target SHALL regenerate the type-safe query layer from the migrations and query directories, so that changing SQL and updating Go code is one step.

#### Scenario: Regenerating after a query change

- **WHEN** a query file is edited and the generate target runs
- **THEN** the generated Go code is updated and the project compiles against it

#### Scenario: Invalid SQL fails the target

- **WHEN** the generate target runs against a query that does not parse against the schema
- **THEN** it fails with the generator's error and leaves the previously generated code unchanged

### Requirement: The Go server hot-reloads on file changes

The Go server SHALL run under a file watcher during development, configured to rebuild and restart the process on source changes and to signal the running process so it can shut down gracefully.

#### Scenario: Saving a Go file restarts the server

- **WHEN** a Go source file is saved while the stack is running
- **THEN** the binary is rebuilt, the old process exits cleanly, and the new process serves subsequent requests

#### Scenario: A build error does not kill the loop

- **WHEN** a Go source file is saved with a compile error
- **THEN** the error is printed and the watcher keeps watching, recovering on the next successful build

#### Scenario: Watcher artifacts are ignored by version control

- **WHEN** the repository status is checked after the watcher has built the binary
- **THEN** the build output directory is ignored

### Requirement: Tests and linting are runnable from the repository root

Make targets SHALL run the Go test suite and the linters for both sides of the project, so verification does not require changing directories or knowing per-language commands.

#### Scenario: Tests run from the root

- **WHEN** the test target runs from the repository root
- **THEN** the Go test suite executes and its result determines the exit status

#### Scenario: Lint covers both languages

- **WHEN** the lint target runs
- **THEN** both the Go code and the frontend code are checked

### Requirement: Generated and vendored artifacts are excluded from version control

`.gitignore` SHALL exclude Node dependency and build output directories, local environment files, and build artifacts, while keeping committed generated sources that the project deliberately tracks.

#### Scenario: Node artifacts are ignored

- **WHEN** dependencies are installed and the frontend is built
- **THEN** neither the dependency directory nor the build output appears as an untracked change

#### Scenario: Deliberately tracked generated files remain tracked

- **WHEN** the repository status is checked
- **THEN** the committed generated route tree and generated query code are still tracked
