# rate-limiting Specification

## Purpose

IP-keyed rate limiting for anonymous operations — a shared limiter interface, in-memory bounded state, trusted-hop client identity, and operator-tunable limits.

## Requirements

### Requirement: A single limiter interface all limiters share

The system SHALL define one rate-limiter interface that answers, for a given key, whether the key may act now and — when it may not — how long the caller must wait before it may. Every rate limiter in the system, including those introduced by later slices, MUST be a registration against this interface rather than a bespoke implementation at the call site. The interface MUST NOT dictate where a given limiter stores its state, so that each limiter's storage decision is local to it and independently reversible.

#### Scenario: Interface answers permission and wait time together

- **WHEN** a caller asks the limiter whether a key may act
- **THEN** the answer is either "allowed", or "refused" accompanied by a duration the caller must wait before retrying

#### Scenario: A new limiter is a registration, not a rewrite

- **WHEN** a later feature needs its own rate limit on a different key
- **THEN** it constructs a limiter against the shared interface with its own configuration and storage choice, without modifying the interface or the existing limiters' call sites

### Requirement: Two anonymous IP-keyed registrations ship in this slice

The system SHALL register exactly two limiters in this slice — one over anonymous clip creation and one over the availability hint — and both MUST be keyed on the client IP. No other limiter is introduced here.

#### Scenario: Anonymous creation is limited by client IP

- **WHEN** a single client IP submits create requests faster than the configured creation limit allows
- **THEN** requests beyond the limit are refused until the client's window permits again

#### Scenario: The availability hint is limited by client IP

- **WHEN** a single client IP calls the availability hint faster than its configured limit allows
- **THEN** calls beyond the limit are refused until the client's window permits again

### Requirement: Limiter state lives in bounded process memory

The two limiters in this slice SHALL hold their state in process memory, and that state MUST be bounded by a configured hard cap on the number of tracked keys, with eviction when the cap is reached. A growing set of distinct keys — from a distributed source or one host walking a large IPv6 allocation — MUST NOT cause unbounded memory growth. Losing this state on process restart is acceptable and MUST NOT be treated as a correctness failure.

#### Scenario: Distinct keys are capped

- **WHEN** more distinct client keys are seen than the configured maximum
- **THEN** the limiter evicts tracked keys to stay within the cap rather than allocating without bound

#### Scenario: State is not durable across restart

- **WHEN** the process restarts
- **THEN** limiter counters reset, and this is expected behaviour rather than an error

### Requirement: Client key is derived from a configured trusted hop

The client IP used as the limiter key SHALL be derived from a forwarded client-IP header only when the request arrives from a hop the deployment is configured to trust; otherwise the direct connection address MUST be used. The forwarded header MUST NEVER be trusted blindly, because a spoofable key is worse than no limiter as it presents the appearance of protection.

#### Scenario: Forwarded header honoured from the trusted hop

- **WHEN** a request arrives from the configured trusted hop carrying a forwarded client-IP header
- **THEN** the limiter keys on the forwarded client IP rather than on the trusted hop's own address

#### Scenario: Forwarded header ignored from an untrusted source

- **WHEN** a request arrives directly (not via the trusted hop) carrying a forwarded client-IP header
- **THEN** the limiter ignores the header and keys on the direct connection address, so the header cannot be used to forge a key

### Requirement: IPv6 keys are bucketed to a prefix

When the derived client IP is IPv6, the limiter key SHALL be the network prefix (a configurable prefix length such as /64), not the full /128 address, so that a single subscriber handed a large allocation cannot obtain effectively unlimited keys while IPv4 clients remain constrained.

#### Scenario: Two addresses in one IPv6 prefix share a key

- **WHEN** two requests arrive from two different /128 addresses within the same configured IPv6 prefix
- **THEN** both count against the same limiter key

#### Scenario: IPv4 addresses are keyed individually

- **WHEN** requests arrive from distinct IPv4 addresses
- **THEN** each is keyed on its own address, unaffected by the IPv6 prefix rule

### Requirement: Limits are configuration

Every tunable of each limiter — requests per window, window length, the memory cap, and the IPv6 prefix length — SHALL be a configuration value with a documented default, so operators can adjust them once the service has seen real traffic without a code change. The wait duration returned on refusal (`Retry-After`) is derived from the active window remainder at refusal time, not a separate configuration value.

#### Scenario: Defaults apply when unset

- **WHEN** the process starts with no limiter tunables set in the environment
- **THEN** each limiter uses its documented default rate, window, cap, and prefix length

#### Scenario: Configured value overrides the default

- **WHEN** a limiter tunable is set in the environment
- **THEN** the limiter uses the configured value in place of its default
