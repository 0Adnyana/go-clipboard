# clips Specification

## Purpose

Anonymous paste and read — create a clip under a name with a fixed two-hour lease, read it back by exact slug over `/api/*`, with exact body round-trip and atomic flat-pool claim.

## Requirements

### Requirement: Create a clip under a name

The system SHALL accept an unauthenticated create request carrying a slug and a text body, persist the clip with a fixed anonymous lifetime of two hours from creation time, and return a success response that includes the slug exactly as stored and expiry. Creation MUST NOT require an account, a session, or a client-chosen lifetime.

#### Scenario: Successful create

- **WHEN** a valid create request is submitted for a free slug with a body within the size cap
- **THEN** the clip is stored, the response indicates success, and the returned slug matches the requested name character-for-character (case preserved)

#### Scenario: Lifetime is fixed at two hours

- **WHEN** a clip is created
- **THEN** its expiry is exactly two hours after creation, with no client-supplied TTL accepted or echoed as a choice

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

### Requirement: Clip HTTP API and OpenAPI stay aligned

Clip create and read SHALL be exposed under `/api/*` as JSON, and `docs/api/openapi.yaml` MUST document those operations, schemas, and error codes in the same change as the handlers so Postman import and curl examples match the running server.

#### Scenario: OpenAPI describes create and read

- **WHEN** `docs/api/openapi.yaml` is imported or inspected after this change
- **THEN** it includes the clip create and read operations with request and response bodies and the conflict / not-found / validation error cases

#### Scenario: Implemented paths match the document

- **WHEN** the documented create and read paths are called through the Caddy origin
- **THEN** the responses match the documented status codes and JSON shapes
