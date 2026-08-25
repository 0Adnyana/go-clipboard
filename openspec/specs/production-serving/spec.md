# production-serving Specification

## Purpose

Production serving — HTTPS at a real hostname via an edge TLS terminator, Host-header enforcement from `PUBLIC_BASE_URL`, the embedded frontend with SPA fallback, and `/api/*` routed by matcher specificity in-process.

## Requirements

### Requirement: Production is served over HTTPS at a real hostname via an edge

Production SHALL be reachable at a configured hostname over HTTPS. `PUBLIC_BASE_URL` SHALL be exactly one absolute HTTPS origin — scheme `https` and a host, with no userinfo, path, query, or fragment — and the public hostname SHALL be derived from it as the single source of truth. TLS termination SHALL occur at an edge proxy in front of the application; the Go server SHALL serve plain HTTP behind that edge. The server MUST reject a request whose `Host` header does not match the derived hostname.

#### Scenario: PUBLIC_BASE_URL is a single absolute HTTPS origin

- **WHEN** the server reads `PUBLIC_BASE_URL` at startup
- **THEN** it accepts the value only if it is exactly one absolute HTTPS origin (scheme `https`, a host, and no userinfo, path, query, or fragment), and it derives the public hostname from that origin

#### Scenario: Traffic is served over HTTPS at the configured hostname

- **WHEN** the configured production hostname is requested over HTTPS
- **THEN** the edge answers with a valid certificate and forwards to the Go server, and no manual certificate step is required of the application process per deploy

#### Scenario: A non-matching Host is rejected

- **WHEN** a request presents a `Host` header other than the hostname derived from `PUBLIC_BASE_URL`
- **THEN** the Go server rejects it, so only the single configured hostname is served

#### Scenario: The application sits behind the edge

- **WHEN** a request reaches the production environment
- **THEN** the edge terminates TLS and the Go server serves both the API and the frontend over plain HTTP behind it

### Requirement: The frontend is embedded in the binary and served with SPA fallback

The compiled frontend assets SHALL be embedded into the server binary at build time and served by the Go server from that embedded filesystem, with a fallback to `index.html` for paths that are neither `/api/*` nor an existing embedded asset, so client-side routes and the `/<slug>` catch-all resolve to the application. The assets served in production are exactly the ones embedded at build time; nothing is read from a separate assets directory or a reverse proxy at runtime.

#### Scenario: An embedded asset is served directly

- **WHEN** a request matches a compiled frontend asset embedded in the binary
- **THEN** the server returns that asset from the embedded filesystem

#### Scenario: An unknown non-API path falls back to the application shell

- **WHEN** a request for a path that is neither `/api/*` nor an existing embedded asset arrives, such as a `/<slug>` route
- **THEN** the server returns the embedded `index.html` so the client application handles the route

#### Scenario: The served assets are the embedded ones

- **WHEN** the running image serves the frontend
- **THEN** the bytes served are the assets embedded into the binary at build time, and no assets are read from the host filesystem or fetched from a proxy

### Requirement: API requests are routed by matcher specificity in-process

Requests whose path begins with `/api/` SHALL be handled by the Go server's API routes with the `/api` prefix intact, and `/api/*` MUST win over the static/SPA-fallback handling by matcher specificity in the server's router rather than by a proxy or by handler declaration order.

#### Scenario: API request is handled with prefix intact

- **WHEN** `GET /api/health` is requested against the production hostname
- **THEN** the server's `/api/health` route handles the request at the path `/api/health` and its response is returned unchanged

#### Scenario: API routing does not depend on registration order

- **WHEN** both the `/api/*` routes and the static/SPA-fallback handler are registered on the same router
- **THEN** `/api/*` is selected by matcher specificity, and changing the order in which handlers are registered does not change which handler serves an API request
