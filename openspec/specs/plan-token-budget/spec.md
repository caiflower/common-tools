## ADDED Requirements

### Requirement: PlanLimiter Allow operation
The system SHALL provide an `Allow` method that atomically checks whether consuming a given number of tokens for a specific model is within the plan budget, and if allowed, deducts the tokens. The method SHALL return whether the request is allowed, the remaining budget, and the total used amount.

#### Scenario: Allow request within budget
- **WHEN** a user requests to consume 1000 tokens for model "gpt4" with weight 3.0 and budget 100000
- **AND** the current sliding window total used is 95000
- **THEN** the request SHALL be allowed (weighted consumption = 3000, total = 98000 ≤ 100000)
- **AND** the remaining budget SHALL be 2000

#### Scenario: Reject request exceeding budget
- **WHEN** a user requests to consume 1000 tokens for model "gpt4" with weight 3.0 and budget 100000
- **AND** the current sliding window total used is 98000
- **THEN** the request SHALL be rejected (weighted consumption = 3000, total = 101000 > 100000)
- **AND** the remaining budget SHALL be 2000
- **AND** no tokens SHALL be deducted

#### Scenario: Sliding window boundary correctness
- **WHEN** a user consumes tokens at time T within window [T-window, T]
- **AND** some buckets fall outside the window at time T+1
- **THEN** only buckets within the current sliding window SHALL be counted toward the budget
- **AND** expired buckets SHALL be cleaned up from Redis

#### Scenario: First request initializes buckets
- **WHEN** a user makes their first request and no Redis key exists
- **THEN** the system SHALL create the key with the current bucket and model fields
- **AND** the request SHALL be allowed if within budget

### Requirement: PlanLimiter Refund operation
The system SHALL provide a `Refund` method that returns previously consumed tokens for a specific model, updating both the bucket data and model statistics. The refund SHALL be weighted by the model's weight factor.

#### Scenario: Refund tokens after LLM generation completes
- **WHEN** a user previously consumed 2000 tokens for "gpt4" (weight 3.0, deducted 6000 weighted)
- **AND** actual usage was 1500 tokens
- **AND** the user refunds 500 tokens
- **THEN** the weighted total SHALL decrease by 1500 (500 × 3.0)
- **AND** the model "gpt4" field SHALL decrease by 500

#### Scenario: Refund does not make total negative
- **WHEN** a user attempts to refund more tokens than were consumed
- **THEN** the total used SHALL be clamped to 0
- **AND** the model field SHALL be clamped to 0

### Requirement: PlanLimiter Usage query
The system SHALL provide a `Usage` method that returns the current plan usage information including total weighted tokens used, remaining budget, window end time, and per-model raw token consumption.

#### Scenario: Query usage with active consumption
- **WHEN** a user queries usage with budget 100000 and has consumed 30000 weighted tokens
- **THEN** the system SHALL return total used = 30000, remaining = 70000
- **AND** SHALL return per-model breakdown (e.g., gpt4=10000, claude=5000)

#### Scenario: Query usage with expired window
- **WHEN** a user queries usage and all buckets are outside the window
- **THEN** the system SHALL return total used = 0, remaining = budget
- **AND** all model fields SHALL be 0

### Requirement: Time-bucketed sliding window algorithm
The system SHALL implement a time-bucketed sliding window where the window duration is divided into fixed-size buckets. Each bucket records the raw token consumption within that time period. The total consumption within any sliding window SHALL be the sum of all buckets whose time range overlaps with the window.

#### Scenario: Bucket aggregation within window
- **WHEN** the window is 8 hours and bucket size is 1 hour
- **AND** tokens were consumed in hours 1, 2, 3, 4, 5, 6, 7, 8
- **THEN** all 8 buckets SHALL be summed for the current window total

#### Scenario: Expired buckets excluded
- **WHEN** the window is 8 hours and current time is T
- **AND** a bucket exists at time T-9h (outside window)
- **THEN** that bucket SHALL NOT be included in the total
- **AND** that bucket SHALL be deleted from Redis

### Requirement: Multi-model token tracking
The system SHALL track token consumption per model using Hash fields prefixed with `_` (e.g., `_gpt4`, `_claude`). These fields store raw (unweighted) token counts for display and analytics purposes.

#### Scenario: Track multiple models
- **WHEN** a user consumes 1000 tokens for "gpt4" and 500 tokens for "claude"
- **THEN** the `_gpt4` field SHALL be 1000 and `_claude` field SHALL be 500
- **AND** the weighted total SHALL reflect each model's weight

### Requirement: Weighted token pricing
The system SHALL support model-specific weight factors. When checking budget, raw tokens SHALL be multiplied by the model's weight to produce weighted tokens. The budget limit SHALL be compared against the sum of weighted tokens.

#### Scenario: Different model weights
- **WHEN** model "gpt4" has weight 3.0 and model "gemini" has weight 0.5
- **AND** a user consumes 1000 tokens for each
- **THEN** gpt4 contributes 3000 weighted tokens and gemini contributes 500 weighted tokens

### Requirement: Configurable window and bucket size
The system SHALL allow configuration of the sliding window duration and bucket size through Option functions. Default values SHALL be window=8 hours, bucket size=1 hour.

#### Scenario: Custom window and bucket configuration
- **WHEN** a PlanLimiter is created with WithWindow(4h) and WithBucketSize(30m)
- **THEN** the sliding window SHALL be 4 hours with 30-minute buckets

### Requirement: Redis key structure and TTL
The system SHALL use a single Redis Hash key per user plan with pattern `plan:{userId}:{planId}`. Bucket fields use timestamp strings as field names. Model fields use `_{modelName}` as field names. The key TTL SHALL be set to window duration + 3600 seconds.

#### Scenario: Key expires after window
- **WHEN** a user becomes inactive
- **THEN** the Redis key SHALL expire after window + 3600 seconds

### Requirement: Lua script atomicity
All Allow, Refund, and Usage operations SHALL be executed as atomic Redis Lua scripts via EVALSHA. Scripts SHALL be pre-loaded on initialization.

#### Scenario: Concurrent requests are serialized
- **WHEN** two concurrent Allow requests arrive for the same user
- **THEN** the Lua script execution SHALL ensure both requests see a consistent state
- **AND** the budget SHALL not be exceeded

### Requirement: Integration tests with miniredis
The system SHALL include comprehensive integration tests using `github.com/alicebob/miniredis/v2` as the Redis server. Tests SHALL cover Allow, Refund, Usage, sliding window boundary, multi-model tracking, and concurrent access scenarios.

#### Scenario: Full integration test suite
- **WHEN** the test suite runs
- **THEN** it SHALL verify Allow accept/reject, Refund correctness, Usage query, window expiration, multi-model stats, and weighted pricing
