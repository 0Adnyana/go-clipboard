# web-client Specification

## Purpose

The frontend application — TanStack Router with a `/$slug` catch-all reflecting the user-owned URL namespace, relative-path API access through the shared origin, and a status view that surfaces backend and database state.

## Requirements

### Requirement: The dev server runs on a pinned port behind the proxy

The frontend dev server SHALL be configured to listen on a fixed port and to fail startup rather than move to another port when that port is occupied. It MUST NOT define its own API proxying, and MUST NOT open a browser window automatically, because the proxy origin is the supported entry point.

#### Scenario: Occupied port fails loudly

- **WHEN** the dev server is started while its configured port is already in use
- **THEN** startup fails with a port error instead of silently binding a different port that the proxy is not forwarding to

#### Scenario: No dev-server-level API proxying exists

- **WHEN** the dev server configuration is inspected
- **THEN** it contains no proxy rules, so API routing exists in exactly one place

#### Scenario: Starting the dev server does not open the wrong URL

- **WHEN** the dev server starts
- **THEN** no browser window is opened automatically to the dev server port

### Requirement: File-based routing with a slug catch-all

The application SHALL use file-based routing with a root route, an index route, and a dynamic single-segment route that captures an arbitrary top-level path, so the user-owned URL namespace is owned by the frontend router.

#### Scenario: Index route renders at the origin root

- **WHEN** the application is loaded at `/`
- **THEN** the index route renders

#### Scenario: Arbitrary top-level path is matched by the slug route

- **WHEN** the application is loaded at `/some-user-key`
- **THEN** the slug route renders with the segment available as a route parameter, rather than the application showing a not-found page

#### Scenario: Slug route holds a placeholder in this change

- **WHEN** the slug route renders
- **THEN** it shows a placeholder view and performs no clipboard behaviour

### Requirement: The generated route tree is committed

The route tree file produced by the router plugin SHALL be committed to version control so a fresh clone type-checks before the dev server has ever been run, and SHALL be excluded from linting and formatting.

#### Scenario: Fresh clone type-checks

- **WHEN** a clean clone runs a type check without first starting the dev server
- **THEN** the check succeeds because the generated route tree is present

#### Scenario: Generated file is not linted

- **WHEN** lint and format run over the frontend
- **THEN** the generated route tree file is skipped

### Requirement: The router plugin is registered before the React plugin

In the build configuration the router plugin SHALL be registered ahead of the React plugin, and the ordering constraint MUST be recorded in a comment at the call site because the wrong order breaks route generation without producing an error.

#### Scenario: Route generation works

- **WHEN** the dev server or build runs
- **THEN** routes are generated from the route files and code splitting is applied

#### Scenario: The constraint is discoverable

- **WHEN** a developer reads the build configuration
- **THEN** a comment states that the router plugin must precede the React plugin and that violating it fails silently

### Requirement: API access uses relative paths through one client module

All backend requests SHALL be issued through a single client module that constructs relative URLs under `/api/`. Components MUST NOT call `fetch` directly and MUST NOT use absolute URLs containing a host or port, so every request stays same-origin through the proxy.

#### Scenario: Request is same-origin

- **WHEN** the application requests the health endpoint
- **THEN** it issues a relative request to `/api/health` against the current origin

#### Scenario: No component bypasses the client

- **WHEN** the frontend source is inspected
- **THEN** network calls originate only from the client module, and no absolute upstream URL appears in application code

#### Scenario: Failed request surfaces as a typed error

- **WHEN** the backend returns a non-success status or an unparseable body
- **THEN** the client raises an error the caller can render, rather than returning a partially-formed value

### Requirement: A status view surfaces backend and database state

The application SHALL include a view that requests the health endpoint and renders the server status, the database reachability with its latency, and whether migrations are pending, proving the path from browser through proxy and server to the database.

#### Scenario: Healthy stack renders positively

- **WHEN** the health endpoint reports everything is fine
- **THEN** the view shows the server, database, and migration state as healthy, including the reported database latency

#### Scenario: Unreachable database is shown as a failure state

- **WHEN** the health endpoint reports the database as unreachable
- **THEN** the view renders a clearly distinguished failure state for the database rather than an empty or blank screen

#### Scenario: Pending migrations are shown as such

- **WHEN** the health endpoint reports pending migrations
- **THEN** the view states that migrations are pending

#### Scenario: Request failure is handled

- **WHEN** the health request itself fails
- **THEN** the view renders an error state instead of remaining in a loading state indefinitely

### Requirement: Styling uses Tailwind v4 through the build plugin

Styling SHALL be provided by Tailwind v4 wired in through its build plugin with a single stylesheet import, with theme customisation expressed in CSS. Legacy JavaScript Tailwind and PostCSS configuration files MUST NOT be present.

#### Scenario: Utility classes are applied

- **WHEN** a component uses Tailwind utility classes
- **THEN** the corresponding styles are applied in the browser

#### Scenario: No legacy config files exist

- **WHEN** the frontend directory is inspected
- **THEN** there is no Tailwind JavaScript config and no PostCSS config file

### Requirement: shadcn/ui components are added through its CLI with a working path alias

The project SHALL be configured for shadcn/ui with a component manifest and an `@/*` path alias declared in both the TypeScript configuration and the build tool's module resolution. Components SHALL be added through the shadcn CLI rather than copied by hand.

#### Scenario: Alias resolves in both type checking and bundling

- **WHEN** a module is imported using the `@/` prefix
- **THEN** the type checker resolves it and the dev server bundles it without a resolution error

#### Scenario: A component added by the CLI works unmodified

- **WHEN** a component is added through the shadcn CLI and used in a route
- **THEN** it renders with its expected styling without manual import rewriting

### Requirement: Frontend dependency versions are pinned

The frontend `package.json` SHALL pin exact versions for its dependencies, so that the fast-moving styling and component libraries cannot change underneath an unchanged lockfile-free install.

#### Scenario: Manifest carries exact versions

- **WHEN** `package.json` is inspected
- **THEN** dependency versions are exact rather than range specifiers
