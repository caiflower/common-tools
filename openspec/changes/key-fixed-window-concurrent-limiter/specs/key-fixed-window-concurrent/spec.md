## ADDED Requirements

### Requirement: Acquire concurrent slot
The system SHALL provide an `Acquire` method that atomically increments the concurrent counter for a given key if the current count is below the configured maximum. The operation SHALL return true if the slot was successfully acquired, and false if the maximum concurrent limit has been reached.

#### Scenario: Acquire when under limit
- **WHEN** a user calls Acquire with key "user:123" and the current concurrent count is 2 with max concurrent 5
- **THEN** the counter SHALL be incremented to 3 and the method SHALL return true

#### Scenario: Acquire when at limit
- **WHEN** a user calls Acquire with key "user:123" and the current concurrent count is 5 with max concurrent 5
- **THEN** the counter SHALL remain at 5 and the method SHALL return false

#### Scenario: Acquire for new key
- **WHEN** a user calls Acquire with key "user:456" that has no existing counter
- **THEN** the counter SHALL be initialized to 1 and the method SHALL return true

### Requirement: Release concurrent slot
The system SHALL provide a `Release` method that atomically decrements the concurrent counter for a given key. The counter SHALL NOT go below 0 to prevent underflow from duplicate releases.

#### Scenario: Release decrements counter
- **WHEN** a user calls Release with key "user:123" and the current concurrent count is 3
- **THEN** the counter SHALL be decremented to 2

#### Scenario: Release prevents underflow
- **WHEN** a user calls Release with key "user:123" and the current concurrent count is 0
- **THEN** the counter SHALL remain at 0 and no error SHALL be returned

#### Scenario: Release for non-existent key
- **WHEN** a user calls Release with key "user:789" that has no existing counter
- **THEN** the counter SHALL remain non-existent (or be set to 0) and no error SHALL be returned

### Requirement: Query current concurrent count
The system SHALL provide a `CurrentConcurrent` method that returns the current concurrent count for a given key.

#### Scenario: Query existing key
- **WHEN** a user calls CurrentConcurrent with key "user:123" and the current count is 3
- **THEN** the method SHALL return 3

#### Scenario: Query non-existent key
- **WHEN** a user calls CurrentConcurrent with key "user:789" that has no existing counter
- **THEN** the method SHALL return 0

### Requirement: Key isolation
Each key SHALL maintain an independent concurrent counter. Operations on one key SHALL NOT affect the counter of any other key.

#### Scenario: Different keys are independent
- **WHEN** key "user:A" has concurrent count 3 and key "user:B" has concurrent count 1
- **THEN** acquiring a slot for "user:B" SHALL only increment "user:B"'s counter, leaving "user:A"'s counter unchanged

### Requirement: Configurable maximum concurrent
The system SHALL allow configuring the maximum concurrent limit both at construction time (as default) and at call time (as per-request override).

#### Scenario: Default max concurrent from construction
- **WHEN** a KeyFixedWindowLimiter is constructed with default max concurrent 5
- **THEN** Acquire calls without explicit override SHALL use max concurrent 5

#### Scenario: Per-request override of max concurrent
- **WHEN** a user calls Acquire with an explicit max concurrent override of 10
- **THEN** the limit check SHALL use 10 instead of the default

### Requirement: Key expiration for leak prevention
The system SHALL set an expiration time (TTL) on each key's counter to prevent permanent slot occupation when Release is not called due to crashes or network issues. The TTL SHALL be refreshed on each Acquire.

#### Scenario: TTL is set on Acquire
- **WHEN** a user calls Acquire for key "user:123"
- **THEN** the key SHALL have a TTL set to the configured expiration duration

#### Scenario: TTL is refreshed on subsequent Acquire
- **WHEN** a user calls Acquire for key "user:123" that already has a TTL
- **THEN** the TTL SHALL be refreshed to the configured expiration duration

### Requirement: Interface compliance
The `KeyFixedWindowLimiter` struct SHALL implement the `KeyFixedWindowLimiterInterface` interface.

#### Scenario: Interface compliance check
- **WHEN** compiling the code
- **THEN** the assignment `var _ KeyFixedWindowLimiterInterface = (*KeyFixedWindowLimiter)(nil)` SHALL compile without error

### Requirement: Configuration validation
The system SHALL validate configuration parameters at construction time and return an error for invalid values.

#### Scenario: Zero or negative max concurrent
- **WHEN** constructing a KeyFixedWindowLimiter with max concurrent <= 0
- **THEN** an error SHALL be returned

#### Scenario: Negative expiration duration
- **WHEN** constructing a KeyFixedWindowLimiter with a negative expiration duration
- **THEN** an error SHALL be returned
