## ADDED Requirements

### Requirement: sarama MockBroker startup and configuration
The integration test framework SHALL use sarama's built-in MockBroker as the test Kafka server. The MockBroker SHALL be configured with appropriate MockResponse mappings for each test case. The MockBroker SHALL be started per test and closed after the test completes.

#### Scenario: MockBroker with Producer responses
- **WHEN** a MockBroker is configured with MetadataResponse and ProduceResponse
- **THEN** a sarama SyncProducer can connect and send messages successfully

#### Scenario: MockBroker with Consumer responses
- **WHEN** a MockBroker is configured with MetadataResponse, FindCoordinatorResponse, OffsetResponse, and FetchResponse
- **THEN** a sarama ConsumerGroup can connect and consume messages

#### Scenario: MockBroker cleanup
- **WHEN** a test finishes
- **THEN** the MockBroker is closed and its TCP listener is released

### Requirement: v2 Producer integration test (MockBroker)
The v2 Producer (sarama) SHALL be tested against a MockBroker for sync and async send operations.

#### Scenario: v2 sync send via MockBroker
- **WHEN** a v2 SyncProducer sends a message to a topic via MockBroker
- **THEN** the send SHALL succeed without error

#### Scenario: v2 async send via MockBroker
- **WHEN** a v2 AsyncProducer sends a message to a topic via MockBroker
- **THEN** the send SHALL succeed without error

### Requirement: v2 Producer idempotence test (mocks package)
The v2 Producer idempotence behavior SHALL be tested using sarama's `mocks` package (interface-level mock), since MockBroker does not support the `InitProducerId` API.

#### Scenario: v2 Producer with idempotence enabled using mocks
- **WHEN** a v2 SyncProducer is created with `ProducerIdempotence=true` using `mocks.NewSyncProducer`
- **THEN** the Producer SHALL accept and process messages with the idempotent configuration

#### Scenario: v2 AsyncProducer with idempotence enabled using mocks
- **WHEN** a v2 AsyncProducer is created with `ProducerIdempotence=true` using `mocks.NewAsyncProducer`
- **THEN** the Producer SHALL accept and process messages with the idempotent configuration

### Requirement: v2 Consumer integration test (MockBroker)
The v2 Consumer (sarama) SHALL be tested against a MockBroker for message consumption.

#### Scenario: v2 ConsumerGroup consumes pre-set messages
- **WHEN** a MockBroker is configured with FetchResponse containing test messages
- **THEN** a v2 ConsumerGroup SHALL consume and process those messages

### Requirement: Build tag isolation
All integration test files SHALL use the `//go:build integration` build tag. Integration tests SHALL NOT run during normal `go test ./...` execution.

#### Scenario: Integration tests excluded from default test run
- **WHEN** `go test ./...` is executed without `-tags=integration`
- **THEN** integration test files are not compiled or run

#### Scenario: Integration tests included with explicit tag
- **WHEN** `go test -tags=integration ./kafka/v2/...` is executed
- **THEN** integration test files are compiled and run
