# http-server Specification

## Purpose

Server lifecycle — typed configuration from the environment, human-readable structured logging via `log/slog`, stdlib `net/http` routing, request-scoped middleware, consistent JSON error responses, and clean shutdown on signal so repeated `air` reloads do not leak ports or connections.
## Requirements
### Requirement: Typed configuration from the environment

The server SHALL read all configuration from environment variables into a single typed configuration struct at startup, applying documented defaults for optional values. The server MUST NOT read or parse `.env` files itself.

#### Scenario: Defaults applied when optional variables are unset

- **WHEN** the process starts with no `PORT` and no migrations directory set in the environment
- **THEN** the configuration resolves `PORT` to `8080` and the migrations directory to `db/migrations`

#### Scenario: Environment values override defaults

- **WHEN** the process starts with `PORT=9090` in the environment
- **THEN** the HTTP server listens on port `9090`

#### Scenario: Missing database connection string is reported at startup

- **WHEN** the process starts with `DATABASE_URL` unset or empty
- **THEN** the process logs a configuration error naming `DATABASE_URL` and exits with a non-zero status

#### Scenario: Malformed configuration value is rejected

- **WHEN** the process starts with `PORT` set to a value that is not a valid port number
- **THEN** the process logs an error identifying the offending variable and exits with a non-zero status rather than falling back to the default

### Requirement: Human-readable structured logging

The server SHALL log through `log/slog` using a handler that emits human-readable text, and every log record MUST carry a level and a message. JSON log output is out of scope for this change.

#### Scenario: Startup is logged with the resolved listen address

- **WHEN** the HTTP server begins listening
- **THEN** an informational record is emitted stating the address and port the server is listening on

#### Scenario: Logger is passed explicitly rather than read from a global

- **WHEN** a handler or middleware needs to log
- **THEN** it uses a logger provided through its dependencies rather than the package-level default logger

### Requirement: API-only routing surface

The server SHALL serve application routes exclusively under the `/api/` path prefix using the standard library `net/http` router with method-qualified patterns. Outside `/api/` the server MAY register a single catch-all whose only behaviour is to return the standard JSON error response, so that a request arriving directly rather than through the proxy is refused in the server's own error format. That catch-all MUST NOT serve static assets, MUST NOT return an HTML document, and MUST NOT implement single-page-application fallback behaviour.

The routing surface SHALL be a fixed property of the binary. The set of registered method-and-path pairs MUST NOT vary with the presence, absence, or nil-ness of any injected dependency, and MUST be identical whether the server is constructed by a test or by the production entrypoint. A missing dependency MUST NOT be expressed as an unregistered route.

Whether a request's path is registered SHALL be decided by the same pattern matcher that resolved the registered routes, for every registered path including those carrying a path parameter. A path MUST NOT be reported as missing on the grounds that the concrete request path differs textually from the pattern that describes it.

#### Scenario: Registered API route is served

- **WHEN** a `GET /api/health` request reaches the server
- **THEN** the server responds with the health payload

#### Scenario: Wrong method on a registered path is rejected

- **WHEN** a `POST /api/health` request reaches the server
- **THEN** the server responds with HTTP 405 and an `Allow` header naming the methods that path answers

#### Scenario: Unknown API path is not reported as a method problem

- **WHEN** a `POST /api/does-not-exist` request reaches the server
- **THEN** the server responds with HTTP 404 rather than 405, because the path is resolved before the method and no path was matched

#### Scenario: Request outside the API prefix is not special-cased

- **WHEN** a request for `/` or `/some-slug` reaches the server directly
- **THEN** the server responds with its own standard not-found response and does not attempt to serve, generate, or fall back to any HTML document, and the response is identical in shape for every such path

#### Scenario: Routing surface does not vary with how the server was constructed

- **WHEN** a server built for a test and a server built by the production entrypoint are compared
- **THEN** both expose the same set of method-and-path pairs, and the fallback resolves the same paths as registered for both

#### Scenario: Every supported path is registered without a runtime condition

- **WHEN** the server is constructed
- **THEN** each API path the product exposes is registered unconditionally, so no supported path can answer with the unmatched-path 404

#### Scenario: Wrong method on a supported path is a method problem, not a missing path

- **WHEN** a `DELETE /api/clips` request reaches the server
- **THEN** the server responds with HTTP 405 and an `Allow` header naming `POST`, rather than 404

#### Scenario: Wrong method on a path carrying a path parameter is also a method problem

- **WHEN** a `DELETE /api/clips/abc123` request reaches the server, where `/api/clips/{slug}` is registered for `GET`
- **THEN** the server responds with HTTP 405 and an `Allow` header naming `GET, HEAD`, rather than 404, because the concrete path matches a registered pattern

#### Scenario: A path deeper than any registered pattern is still unknown

- **WHEN** a request reaches a path one segment below a registered pattern, such as `/api/clips/abc123/extra`
- **THEN** the server responds with HTTP 404, because a path parameter matches a single segment and no registered pattern matches the request

### Requirement: Consistent JSON error responses

Every error response the server produces SHALL have a `Content-Type` of `application/json` and a body of a consistent shape carrying a machine-readable error code and a human-readable message. The server MUST NOT emit the standard library's plain-text error bodies to clients.

#### Scenario: Unmatched API path returns a JSON not-found

- **WHEN** a request for `/api/does-not-exist` reaches the server
- **THEN** the response has status 404, `Content-Type: application/json`, and a body containing an error code and message

#### Scenario: Unmatched path outside the API prefix also returns JSON

- **WHEN** a request for `/some-slug` reaches the server directly rather than through the proxy
- **THEN** the response has status 404 with `Content-Type: application/json`, and the standard library's plain-text `404 page not found` body is never emitted

#### Scenario: Internal failure does not leak implementation detail

- **WHEN** a handler fails with an unexpected internal error
- **THEN** the response has status 500 with a generic JSON error body, and the underlying error is recorded in the server log instead of the response

### Requirement: Request-scoped middleware

The server SHALL apply middleware written as `func(http.Handler) http.Handler`, composed in one place, so that every API request is logged on completion with its method, path, status code, and duration, and so that a panic in a handler is recovered without terminating the process.

#### Scenario: Completed request is logged once

- **WHEN** a request to any API route completes
- **THEN** exactly one log record is emitted for it containing the method, path, response status, and elapsed duration

#### Scenario: Panic in a handler is contained

- **WHEN** a handler panics while serving a request
- **THEN** the middleware recovers, logs the panic, responds with the standard JSON 500 error, and the process keeps serving subsequent requests

### Requirement: Health endpoint reports stack state

The server SHALL expose `GET /api/health` returning a JSON body that reports overall status, database reachability with an observed latency, and whether migrations are pending together with the current schema version. The endpoint SHALL return HTTP 200 whenever the process is able to answer, and MUST report degraded dependencies in the body rather than through the status code. Every value the body reports MUST have been observed: where a check could not complete, the endpoint MUST omit that check's state rather than emit a default that reads as a successful observation.

The dependencies this endpoint observes — the connection pool, the migration checker, and the query handle — are the server's only optional dependencies. Each MAY be absent, and absence MUST be reported as a degraded check rather than prevent the server from starting. No other dependency may be treated as optional on the grounds that this one is.

#### Scenario: Everything is wired up

- **WHEN** the database is reachable and no migrations are pending
- **THEN** `GET /api/health` returns 200 with an overall status of `ok`, the database marked reachable with a latency value, and migrations marked not pending with the current version

#### Scenario: Database is unreachable

- **WHEN** Postgres is not running or refuses connections
- **THEN** `GET /api/health` still returns 200, the database is marked unreachable, and the overall status indicates the stack is not healthy

#### Scenario: Migrations have not been applied

- **WHEN** the database is reachable but unapplied migration files exist
- **THEN** `GET /api/health` returns 200 with migrations marked pending, and the check does not apply them

#### Scenario: Migration state cannot be determined

- **WHEN** the pending-migration check cannot complete, for example because the database is unreachable
- **THEN** `GET /api/health` returns 200 with the overall status not healthy and reports no migration state at all, rather than reporting migrations as not pending at schema version zero

#### Scenario: An absent health dependency does not prevent startup

- **WHEN** the migration runner cannot be constructed at startup
- **THEN** the process still starts and serves requests, and `GET /api/health` returns 200 reporting no migration state, rather than refusing to start

### Requirement: Graceful shutdown on signal

On receiving `SIGINT` or `SIGTERM` the server SHALL stop accepting new connections, allow in-flight requests to finish within a bounded timeout, and release its resources in an order that closes the database pool explicitly and last.

#### Scenario: Signal releases the listening port

- **WHEN** the process receives `SIGTERM`
- **THEN** the process exits cleanly and the configured port is immediately available, so a restart does not fail with an address-already-in-use error

#### Scenario: In-flight request is allowed to finish

- **WHEN** the process receives `SIGTERM` while a request is being served
- **THEN** the server finishes that response before exiting, unless the shutdown timeout elapses first

#### Scenario: Database pool is closed on shutdown

- **WHEN** the process shuts down after a signal
- **THEN** the connection pool is closed explicitly so no Postgres connections are left open by the exiting process

### Requirement: Entrypoint command dispatch

The `cmd/server` entrypoint SHALL select behaviour from its first command-line argument: no argument or `serve` starts the HTTP server, and recognised schema commands operate on the database and exit. An unrecognised command MUST be rejected with usage output and a non-zero exit status rather than silently starting the server.

#### Scenario: Default invocation starts the server

- **WHEN** the binary is run with no arguments
- **THEN** the HTTP server starts and listens on the configured port

#### Scenario: Unknown subcommand is rejected

- **WHEN** the binary is run with an unrecognised first argument
- **THEN** it prints the available commands and exits with a non-zero status without starting the HTTP server

### Requirement: Required dependencies are resolved before the server is built

The process SHALL construct domain services in its entrypoint and pass them to the HTTP server as required dependencies. The server constructor MUST NOT assemble domain services or their storage adapters, and MUST NOT accept two different fields as alternate ways to supply the same capability. When a required dependency is absent, construction MUST fail with an error naming every missing dependency, and the process MUST log that failure and exit with a non-zero status rather than serve a reduced surface.

#### Scenario: Missing required dependency is refused at construction

- **WHEN** the HTTP server is constructed without a required domain service
- **THEN** construction fails with an error naming the missing dependency and no server is returned

#### Scenario: Every missing dependency is named at once

- **WHEN** the HTTP server is constructed with more than one required dependency absent
- **THEN** the reported error names all of them, so wiring is not repaired one restart at a time

#### Scenario: Entrypoint exits rather than listening with a reduced surface

- **WHEN** the process cannot supply a required dependency to the server
- **THEN** it logs the failure and exits with a non-zero status without binding the listening port

#### Scenario: Service assembly lives in the entrypoint

- **WHEN** a domain service needs a database-backed store
- **THEN** the entrypoint builds the store and the service and passes the service to the server, and the HTTP layer constructs neither

### Requirement: Rate-limit refusal is a standard 429 in the JSON error shape

When a request is refused by a rate limiter the server SHALL respond with HTTP 429, a `Retry-After` header stating how long the caller must wait (derived from the active window remainder), and a JSON body in the same consistent error shape every other error uses with error code `rate_limited` and message `too many requests`. The response MUST NOT reveal the limiter's key, its capacity, its rate, its window, or any other limiter metadata in the body or headers beyond the wait duration.

#### Scenario: Refused request returns 429 with Retry-After and rate_limited

- **WHEN** a request exceeds a limiter's allowance
- **THEN** the response has status 429, a `Retry-After` header with the wait duration, and a JSON body with error code `rate_limited` and message `too many requests`

#### Scenario: Refusal leaks nothing about the limit

- **WHEN** a 429 refusal is returned
- **THEN** the body and headers disclose only how long to wait, and nothing about the limiter's key, capacity, rate, or window

### Requirement: Client IP is derived from a configured trusted hop

The server SHALL derive the client IP for rate limiting from the `X-Forwarded-For` header only when the immediate connection comes from the hop configured as `TRUSTED_PROXY`, and otherwise from the direct connection address. Which hop is trusted MUST be read from configuration, because it changes with the deployment topology, and `X-Forwarded-For` MUST NEVER be trusted from an untrusted source.

#### Scenario: Trusted proxy's forwarded IP is used

- **WHEN** a request arrives from the configured trusted hop with an `X-Forwarded-For` header
- **THEN** the server uses the right-most valid forwarded client IP as the client identity for rate limiting

#### Scenario: Direct connection address is used when the hop is untrusted

- **WHEN** a request arrives directly from a source not configured as trusted, carrying an `X-Forwarded-For` header
- **THEN** the server ignores the header and uses the direct connection address

### Requirement: Rate-limiter configuration is read from the environment

The server SHALL read the trusted-hop setting and every limiter tunable (rate, window, memory cap, IPv6 prefix length) from environment variables into the typed configuration struct at startup, applying documented defaults for values that are unset, consistent with how all other configuration is loaded. Startup validation MUST reject invalid values: `TRUSTED_PROXY` must be a parseable IP address when set; each rate and `RATELIMIT_MAX_KEYS` must be a positive integer when set; each window must be a positive duration when set; and `IPV6_PREFIX` must be an integer from 0 through 128 when set. The error logged on rejection MUST name the offending environment variable.

#### Scenario: Defaults applied when limiter variables are unset

- **WHEN** the process starts with no rate-limiter variables set
- **THEN** the configuration resolves each limiter tunable to its documented default and starts normally

#### Scenario: Malformed limiter value is rejected at startup

- **WHEN** the process starts with a rate-limiter variable set to a value that fails semantic validation
- **THEN** the process logs an error identifying the offending environment variable and exits with a non-zero status rather than falling back to the default

