# clip-sweeper Specification

## Purpose
TBD - created by archiving change lifetime-you-can-see. Update Purpose after archive.
## Requirements
### Requirement: A background sweeper deletes expired rows

The application SHALL run a background process that periodically deletes clip rows whose expiry is in the past, bounding how long dead rows accumulate in the table. The sweep interval SHALL be configurable, and each sweep SHALL log the number of rows removed at a level suitable for routine operation.

#### Scenario: Expired rows are removed on the interval

- **WHEN** clips have expired and a sweep runs
- **THEN** the expired rows are deleted from the table and the count removed is logged

#### Scenario: Live rows are never swept

- **WHEN** a sweep runs while some clips are still live
- **THEN** only rows whose expiry is in the past are deleted and every live clip remains readable

### Requirement: Sweeper is housekeeping, never a correctness dependency

Deletion of expired rows by the sweeper SHALL be pure housekeeping. Stopping, disabling, or failing the sweeper MUST NOT change the observable behaviour of claim or read: an expired row is already non-live to reads and freely reclaimable by the atomic claim path whether or not the sweeper has removed it yet.

#### Scenario: Reads and claims are correct with the sweeper stopped

- **WHEN** the sweeper is not running and an expired row still sits in the table
- **THEN** a read of that slug is not-found and a create for that slug succeeds by reclaiming the expired row, exactly as if the row had been swept

#### Scenario: A sweep failure does not crash the server

- **WHEN** a single sweep pass fails (for example the database is briefly unreachable)
- **THEN** the failure is logged and the server keeps serving requests, and the next scheduled sweep proceeds normally

### Requirement: Sweeper lifecycle is tied to server startup and shutdown

The sweeper SHALL be started by the same wiring that starts the HTTP server and SHALL stop cleanly on shutdown, so it does not outlive the process or block graceful shutdown beyond the shutdown timeout.

#### Scenario: Sweeper starts with the server

- **WHEN** the server process starts in serve mode
- **THEN** the sweeper begins running on its configured interval

#### Scenario: Sweeper stops on shutdown

- **WHEN** the process receives a shutdown signal
- **THEN** the sweeper stops within the shutdown window and the process exits cleanly without a leaked goroutine or an in-flight sweep blocking exit indefinitely

