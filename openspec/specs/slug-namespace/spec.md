# slug-namespace Specification

## Purpose

Shared rules for clip names — case-sensitive matching, charset and length bounds, and a single reserved-name list so app routes are never claimable as clips.

## Requirements

### Requirement: Slug charset and length bounds

A claimable slug SHALL consist only of ASCII letters, digits, underscore, and hyphen (`[a-zA-Z0-9_-]`), with length between 3 and 64 characters inclusive. Names outside that charset or length MUST be rejected at create time with a validation error. One- and two-character names MUST NOT be claimable.

#### Scenario: Valid slug is accepted

- **WHEN** a create request uses a slug of 3–64 characters drawn only from `[a-zA-Z0-9_-]`
- **THEN** slug validation passes and claim logic proceeds

#### Scenario: Too-short slug is rejected

- **WHEN** a create request uses a one- or two-character slug
- **THEN** the server responds with a validation error and stores nothing

#### Scenario: Illegal character is rejected

- **WHEN** a create request uses a slug containing a character outside `[a-zA-Z0-9_-]`
- **THEN** the server responds with a validation error and stores nothing

### Requirement: Names are case-sensitive

Slug comparison and storage SHALL treat letter case as significant: `/Notes` and `/notes` are different clips. The stored form MUST preserve the exact casing submitted at create.

#### Scenario: Mixed-case create preserves casing

- **WHEN** a clip is created with slug `Notes`
- **THEN** it is stored and returned as `Notes`

#### Scenario: Different casing does not find the clip

- **WHEN** a live clip exists at `notes` and it is requested as `Notes`
- **THEN** the read is not-found unless a separate clip was created at `Notes`

#### Scenario: Case variants do not collide

- **WHEN** a live clip holds `notes` and a create is attempted for `Notes`
- **THEN** the create succeeds as a distinct claim

### Requirement: Reserved names come from one list

The system SHALL maintain a single reserved-name list of top-level path segments that must not be claimable as clips because they belong to real application pages or assets. Create MUST refuse any slug that exactly matches a reserved name (case-sensitive). Frontend routes that occupy top-level segments MUST be added to that same list rather than maintained separately.

#### Scenario: Reserved slug cannot be claimed

- **WHEN** a create request uses a slug present on the reserved list
- **THEN** the server responds with a validation error and stores nothing

#### Scenario: List has a single source

- **WHEN** a new top-level application page is introduced in a later change
- **THEN** its segment is added to the same reserved-name list consulted by create, not to a second ad-hoc list
