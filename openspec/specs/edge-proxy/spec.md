# edge-proxy Specification

## Purpose

The single browser origin — one entry point routing `/api/*` to the Go server and everything else to the frontend, with WebSocket upgrade preserved for hot module replacement, path precedence expressed in configuration rather than application code, and a diagnosable response when an upstream is not running.

## Requirements

### Requirement: A single browser origin fronts every upstream

Development SHALL expose one HTTP origin, served by a reverse proxy defined in a `Caddyfile` at the repository root, through which both the API and the frontend are reachable. The proxy origin is the supported entry point; the individual upstream ports MUST NOT be required for normal development.

#### Scenario: API and application share one origin

- **WHEN** the developer loads the application and it issues an API request from the proxy origin
- **THEN** both the document and the API response come from the same scheme, host, and port, so the request is same-origin and no CORS handling is involved anywhere

#### Scenario: Routing rules live in configuration

- **WHEN** the routing between API and application traffic needs to change
- **THEN** it is changed in the `Caddyfile`, and no application code is modified

### Requirement: API requests are routed to the Go server with the prefix intact

Requests whose path begins with `/api/` SHALL be forwarded to the Go server, and the `/api` prefix MUST be preserved in the forwarded request because the server's own routes include it.

#### Scenario: Health request reaches the Go server through the proxy

- **WHEN** `GET /api/health` is requested from the proxy origin
- **THEN** the Go server's health response is returned unchanged

#### Scenario: Prefix is not stripped

- **WHEN** the proxy forwards a request for `/api/health`
- **THEN** the Go server receives the path `/api/health` rather than `/health`

#### Scenario: Unmatched API path is answered by the Go server

- **WHEN** `GET /api/nope` is requested from the proxy origin
- **THEN** the response is the Go server's JSON 404, not an HTML document from the frontend

### Requirement: All other requests are routed to the frontend

Requests that do not begin with `/api/` SHALL be forwarded to the Vite dev server, including requests for arbitrary top-level paths that represent the user-owned URL namespace.

#### Scenario: Application document is served through the proxy

- **WHEN** `/` is requested from the proxy origin
- **THEN** the frontend document is returned

#### Scenario: Arbitrary top-level path renders the application

- **WHEN** `/some-user-key` is requested from the proxy origin
- **THEN** the frontend document is returned rather than a 404, so the frontend router can handle the path

### Requirement: Path precedence does not depend on declaration order

The proxy configuration SHALL express API and catch-all routing as mutually exclusive handlers resolved by matcher specificity, so that the API route always wins over the catch-all regardless of the order the blocks appear in the file.

#### Scenario: API route wins over the catch-all

- **WHEN** a request path matches both the API matcher and the catch-all
- **THEN** it is handled by the API route

#### Scenario: Reordering the configuration does not change behaviour

- **WHEN** the handler blocks in the `Caddyfile` are reordered
- **THEN** requests are routed identically

### Requirement: WebSocket upgrades are preserved for hot module replacement

The proxy SHALL forward WebSocket upgrade requests to the frontend upstream and SHALL pass the original `Host` header through, so the dev server's client computes a same-origin WebSocket URL that routes back through the proxy.

#### Scenario: Editing a component updates the browser without a full reload

- **WHEN** a frontend source file is saved while the application is open at the proxy origin
- **THEN** the change is applied by hot module replacement rather than by a full page refresh

#### Scenario: Upgrade request is not answered as a plain HTTP request

- **WHEN** the browser opens the hot-reload WebSocket connection through the proxy origin
- **THEN** the connection is upgraded and stays open

### Requirement: Proxy ports come from the environment

The proxy listen port and both upstream ports SHALL be read from environment variables with defaults at configuration load, using the same variables that configure the Go server and the dev server, so that no port is written down in more than one place.

#### Scenario: Changing a port in one place is enough

- **WHEN** the API port is changed in the environment file and the stack is restarted
- **THEN** the Go server listens on the new port and the proxy forwards API traffic to it without any edit to the `Caddyfile`

#### Scenario: Defaults apply when the environment is empty

- **WHEN** the proxy starts with none of the port variables set
- **THEN** it listens on the default proxy port and forwards to the default API and frontend ports

### Requirement: The development origin is plain HTTP

The site address in the proxy configuration SHALL specify the `http` scheme explicitly so that automatic HTTPS and the local certificate authority are not activated, and first run requires no privileged trust step.

#### Scenario: First run needs no certificate trust

- **WHEN** the proxy is started for the first time on a machine that has never run it
- **THEN** it serves plain HTTP immediately without prompting for elevated privileges and without a TLS handshake failure in the browser

### Requirement: API upstream restarts are absorbed

The API upstream SHALL be configured to retry connection attempts for a bounded period, so that a request issued while the Go process is being rebuilt and restarted completes rather than failing.

#### Scenario: Request during a rebuild still succeeds

- **WHEN** an API request arrives during the brief window in which the Go process is restarting after a file save
- **THEN** the proxy retries until the upstream accepts the connection and the request completes normally

#### Scenario: Retry is bounded

- **WHEN** the API upstream stays unavailable beyond the retry window
- **THEN** the proxy stops retrying and returns an error response rather than holding the request open indefinitely

### Requirement: An unavailable upstream is diagnosable

When an upstream cannot be reached the proxy SHALL return a gateway error and SHALL log the upstream address it failed to dial, and the documentation MUST state how to tell which process is missing from the failing request path.

#### Scenario: Go server is not running

- **WHEN** an API request is made through the proxy while the Go server is stopped
- **THEN** the proxy returns a gateway error and logs the API upstream address it failed to reach

#### Scenario: Frontend dev server is not running

- **WHEN** a page is loaded through the proxy while the dev server is stopped
- **THEN** the proxy returns a gateway error and logs the frontend upstream address it failed to reach

### Requirement: The proxy sets X-Forwarded-For for the API

The proxy SHALL set the `X-Forwarded-For` header on requests it forwards to the Go server under `/api/*`, so the server can derive the client identity for rate limiting from a known, trusted hop configured as `TRUSTED_PROXY`. The proxy MUST overwrite any client-supplied `X-Forwarded-For` value with the address it observed for the client, so a client cannot forge the identity the server rate-limits on. When multiple entries are present in the header the Go server receives, it SHALL use the right-most parseable IP address as the client identity, ignoring invalid entries.

#### Scenario: API request carries the client IP to the server

- **WHEN** a request to `/api/*` is forwarded from the proxy to the Go server
- **THEN** it carries an `X-Forwarded-For` header set by the proxy to the address the proxy observed for the client

#### Scenario: Client-supplied X-Forwarded-For does not pass through unchanged

- **WHEN** a client sends its own `X-Forwarded-For` header to the proxy
- **THEN** the proxy replaces it with the observed client address, so a client cannot forge the identity the server rate-limits on

#### Scenario: Server uses the right-most valid forwarded IP

- **WHEN** the Go server receives a request from the configured trusted hop with `X-Forwarded-For` containing multiple comma-separated values
- **THEN** it uses the right-most parseable IP address as the client identity for rate limiting

#### Scenario: Invalid forwarded entries are skipped

- **WHEN** the Go server receives `X-Forwarded-For` containing invalid entries alongside valid ones
- **THEN** it skips invalid entries and uses the right-most valid IP address, or falls back to the direct connection address if none are valid
