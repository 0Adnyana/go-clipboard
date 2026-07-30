# web-client Specification

## Purpose

The frontend application — TanStack Router with a `/$slug` catch-all reflecting the user-owned URL namespace, relative-path API access through the shared origin, a create form on `/`, clip read pages on `/$slug`, and a status view at `/status`.
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
- **THEN** the index route renders the create form

#### Scenario: Arbitrary top-level path is matched by the slug route

- **WHEN** the application is loaded at `/some-user-key`
- **THEN** the slug route renders with the segment available as a route parameter, rather than the application showing a not-found page

#### Scenario: Slug route performs clipboard read

- **WHEN** the slug route renders for a path segment
- **THEN** it attempts to load and display the live clip for that slug rather than showing a non-functional placeholder

### Requirement: Index route is the create form

The index route SHALL present a create form that accepts a name, a body, and a chosen lifetime, submits them through the shared API client, and on success navigates to the new clip's read URL. The form MUST include a plain-language note near the name field explaining that a guessable name is effectively a bulletin board and a hard-to-guess name is effectively a secret link. The lifetime control SHALL offer the preset set (leaning 10 minutes / 1 hour / 2 hours), default to the documented default preset, and be framed as *the anonymous limit* rather than *the* limit, so longer signed-in presets can be added later without rewording. The form SHALL show an advisory availability hint near the name field as the user types, making clear the hint is advisory and does not reserve the name.

#### Scenario: Successful paste navigates to the clip

- **WHEN** the user submits a valid name, body, and lifetime from the create form
- **THEN** the client creates the clip via `/api/` and navigates to `/<slug>` using the slug returned by the API (case preserved)

#### Scenario: Exposure note is visible before submit

- **WHEN** the create form is shown
- **THEN** a plain-language note near the name field describes the bulletin-board versus secret-link nature of the chosen name

#### Scenario: Lifetime presets are shown and framed as the anonymous limit

- **WHEN** the create form is shown
- **THEN** the lifetime presets are selectable, one is preselected as the default, and the copy frames the ceiling as the anonymous limit rather than an absolute limit

#### Scenario: Availability hint is advisory

- **WHEN** the user types a name that is currently taken
- **THEN** the form shows an advisory hint that the name looks unavailable, without blocking submission and without claiming the name, and a later create still surfaces an authoritative in-use error if it loses the race

#### Scenario: In-use name is shown as such

- **WHEN** create fails because the name is in use
- **THEN** the form shows that the name is in use right now without implying who holds it

### Requirement: Slug route is the read page with faithful copy

The `/$slug` route SHALL load the live clip for its path parameter and display the body without trimming or re-wrapping the source for storage purposes. Wrapping, if any, is display-only. A single control MUST copy the source string returned by the API (or an equivalent in-memory copy of it), not text scraped from the DOM. The read page SHALL show a live countdown to expiry driven by the server's reported expiry and current time rather than a timer started at page load, and transition to a distinct expired state when the countdown reaches zero.

#### Scenario: Live clip renders its body

- **WHEN** the user opens `/notes` for a live clip
- **THEN** the page shows the clip body with whitespace preserved visually enough to read, without altering the underlying source string

#### Scenario: Copy uses the source string

- **WHEN** the user activates the copy control
- **THEN** the clipboard receives the exact source string from the API response, including trailing newlines and tabs

#### Scenario: Countdown is driven by the server's expiry

- **WHEN** a live clip is opened and time passes on the page
- **THEN** the displayed remaining time ticks down toward the server's expiry, computed against the server's clock so a skewed browser clock does not make the countdown wrong, rather than counting down from a duration fixed at page load

#### Scenario: Expired clip shows a distinct expired state

- **WHEN** the countdown reaches zero, or the clip is already expired on load
- **THEN** the page shows a clearly distinguished expired state rather than a blank body or a stuck countdown

#### Scenario: Missing clip is a not-found state

- **WHEN** the user opens a slug with no live clip
- **THEN** the page shows a not-found state rather than an empty body that looks like a blank paste

### Requirement: Clip pages and robots exclude indexing

The application SHALL serve a `robots.txt` that disallows all crawlers, and clip read pages SHALL carry a `noindex` robots meta directive so pasted content is not treated as a searchable corpus.

#### Scenario: robots.txt disallows everything

- **WHEN** `/robots.txt` is requested
- **THEN** the response instructs crawlers to disallow all paths

#### Scenario: Read page is marked noindex

- **WHEN** a clip read page is rendered
- **THEN** the document includes a robots meta directive that includes `noindex`

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

