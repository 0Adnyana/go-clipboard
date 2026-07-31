# clips Specification

## Purpose

Anonymous paste and read — create a clip under a name with a fixed two-hour lease, read it back by exact slug over `/api/*`, with exact body round-trip and atomic flat-pool claim.
## Requirements
### Requirement: Create a clip under a name

The system SHALL accept an unauthenticated create request carrying a slug, a text body, and a chosen lifetime drawn from a fixed preset set, persist the clip with its expiry anchored absolutely to creation time (`expires_at = created_at + ttl`), and return a success response that includes the slug exactly as stored and the resulting expiry. Creation MUST NOT require an account or a session. Every preset offered to anonymous callers MUST be within the permanent two-hour anonymous ceiling, and a request that omits a lifetime MUST fall back to a documented default preset. A request carrying a lifetime outside the accepted preset set MUST be rejected with a validation error.

#### Scenario: Successful create with a chosen preset

- **WHEN** a valid create request is submitted for a free slug, with a body within the size cap and a lifetime from the preset set
- **THEN** the clip is stored, the response indicates success, the returned slug matches the requested name character-for-character (case preserved), and the returned expiry equals creation time plus the chosen preset

#### Scenario: Lifetime is anchored absolutely to creation

- **WHEN** a clip is created with a chosen lifetime
- **THEN** its expiry is exactly that lifetime after creation time, and no later read, poll, or overwrite of a still-live clip moves that expiry (no sliding TTL, no reset-on-write)

#### Scenario: Omitted lifetime uses the default preset

- **WHEN** a create request does not specify a lifetime
- **THEN** the clip is stored with the documented default preset applied and its expiry reflects that default

#### Scenario: Out-of-range lifetime is rejected

- **WHEN** a create request specifies a lifetime that is not one of the accepted presets, or that exceeds the two-hour anonymous ceiling
- **THEN** the server responds with a validation error and stores nothing

### Requirement: Read a live clip by slug

The system SHALL return the stored body of a live clip when requested by the exact slug. A missing or expired clip MUST produce a not-found response that does not distinguish those two cases.

#### Scenario: Live clip is returned verbatim

- **WHEN** a live clip is requested by the same slug string used at create
- **THEN** the response includes the body exactly as stored, including leading and trailing whitespace, tabs, and newlines

#### Scenario: Different casing is a different name

- **WHEN** a live clip exists at `notes` and it is requested as `Notes`
- **THEN** the read response is not-found (unless a separate live clip was created at `Notes`)

#### Scenario: Missing and expired look the same

- **WHEN** a slug has never been claimed, or its clip has expired
- **THEN** the read response is a not-found with the same shape in both cases

### Requirement: Text round-trips exactly

Stored clip bodies SHALL be persisted and returned without trimming, Unicode normalisation, line-ending conversion, or HTML entity encoding. The create path MUST reject payloads that cannot be represented and returned unchanged, rather than storing a mangled substitute.

#### Scenario: Whitespace and control characters survive

- **WHEN** a body containing leading spaces, trailing newlines, and tab characters is created and then read
- **THEN** the read body is byte-for-byte identical to the created body

#### Scenario: Non-round-trippable payload is refused

- **WHEN** a create request body cannot be decoded as the expected text representation
- **THEN** the server responds with a client error naming the problem and stores nothing

### Requirement: Size cap is enforced before buffering the body

The server SHALL refuse create bodies larger than the documented maximum (256 KiB) and MUST enforce that limit before reading the full request body into memory.

#### Scenario: Oversized body is rejected

- **WHEN** a create request declares or delivers a body larger than 256 KiB
- **THEN** the server responds with a client error and does not persist a clip

#### Scenario: Body at the cap is accepted

- **WHEN** a create request delivers a body of exactly 256 KiB of valid text
- **THEN** the clip is stored successfully

### Requirement: Name claim is atomic and is not an edit

Claiming a slug SHALL be a single atomic operation that succeeds only when no live clip holds that name. A live collision MUST be refused with a message that the name is in use right now, without identifying the holder. A successful claim MUST NOT overwrite the body of a still-live clip.

#### Scenario: Live name is refused

- **WHEN** a create is attempted for a slug held by a live clip
- **THEN** the server refuses with a conflict response whose message states that the name is in use right now and reveals nothing else about the existing clip

#### Scenario: Expired name is reclaimable

- **WHEN** a create is attempted for a slug whose only prior clip has expired
- **THEN** the claim succeeds and the new body and expiry replace the expired lease

#### Scenario: Concurrent claims do not both win

- **WHEN** two create requests for the same free slug are processed concurrently
- **THEN** exactly one succeeds and the other is refused as in use

### Requirement: Anonymous creation and availability are rate-limited

The anonymous create operation and the availability hint SHALL each be protected by an IP-keyed rate limiter. When a client exceeds a limiter's allowance the operation MUST be refused with the standard 429 response carrying `Retry-After`, and MUST NOT perform its work — a refused create stores nothing and a refused availability check reveals nothing. Refusal MUST NOT weaken the correctness of the claim or read paths.

#### Scenario: Excessive creation from one client is refused

- **WHEN** a single client IP submits create requests beyond the configured creation limit
- **THEN** the excess requests are refused with 429 and `Retry-After`, no clip is stored for the refused requests, and the namespace does not grow from them

#### Scenario: Excessive availability checks from one client are refused

- **WHEN** a single client IP calls the availability hint beyond its configured limit
- **THEN** the excess calls are refused with 429 and `Retry-After` and disclose nothing about any name

#### Scenario: Limiting does not break claim or read

- **WHEN** a client stays within its limit
- **THEN** create, read, and availability behave exactly as before, and claim atomicity and read not-found semantics are unchanged

### Requirement: Clip HTTP API and OpenAPI stay aligned

Clip create, read, and availability SHALL be exposed under `/api/*` as JSON, and `docs/api/openapi.yaml` MUST document those operations, schemas, and error codes — including the rate-limit refusal (HTTP 429 with `Retry-After`) — in the same change as the handlers so Postman import and curl examples match the running server.

#### Scenario: OpenAPI describes create, read, and availability

- **WHEN** `docs/api/openapi.yaml` is imported or inspected after this change
- **THEN** it includes the clip create (with its lifetime field), read, and availability operations with request and response bodies, and the conflict / not-found / gone / validation / rate-limit (429) error cases

#### Scenario: Implemented paths match the document

- **WHEN** the documented create, read, and availability paths are called through the Caddy origin
- **THEN** the responses match the documented status codes and JSON shapes, including a 429 with `Retry-After` when a limit is exceeded

### Requirement: Advisory availability check

The system SHALL expose a read-only operation that reports whether a given name currently appears claimable, intended to warn a user before they compose a long body under a taken name. The check MUST apply the same charset, length, and reserved-name rules as create, so a name that could never be claimed is never reported available. The check is advisory only: it MUST NOT reserve, lock, or otherwise hold the name, and a positive result MUST NOT weaken the atomicity of the claim path — two callers both told "available" MUST still race into the claim with exactly one winner.

#### Scenario: Free name is reported available

- **WHEN** the availability of a validly-formed, unreserved name with no live clip is requested
- **THEN** the response reports the name as currently available

#### Scenario: Live name is reported unavailable

- **WHEN** the availability of a name held by a live clip is requested
- **THEN** the response reports the name as not available, without revealing anything about the holder

#### Scenario: Invalid or reserved name is reported unavailable, not available

- **WHEN** the availability of a name that fails charset/length rules or matches the reserved list is requested
- **THEN** the response reports the name as unavailable (or a validation error), never as available

#### Scenario: Availability does not reserve the name

- **WHEN** a name is reported available and two creates then race for it
- **THEN** exactly one create succeeds and the other is refused as in use, exactly as if the availability check had never run

