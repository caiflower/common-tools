## Why

Kafka 模块（v2/sarama）目前只有单元测试，没有集成测试验证 Producer/Consumer 的端到端交互。新增的 `ProducerIdempotence` 配置也需要在更真实的环境下验证其行为。使用 sarama 自带的 MockBroker 可以在测试中启动一个真实的 TCP Kafka 协议服务器，无需 Docker 依赖，提供轻量、可重复的集成测试环境。

## What Changes

- 使用 sarama 内置的 `MockBroker` 和 `mocks` 包作为测试基础设施，零额外依赖
- 新增 v2 的集成测试文件，覆盖 Producer 同步/异步发送、Consumer 消费等核心场景
- 测试通过 build tag `integration` 隔离，不影响日常单元测试

## Capabilities

### New Capabilities
- `kafka-v2-integration-test`: 基于 sarama MockBroker 的 Kafka v2 集成测试框架，包含 MockBroker 启动、MockResponse 配置、Producer 和 Consumer 的端到端验证用例

### Modified Capabilities

## Impact

- 无新增外部依赖（sarama 已是项目依赖）
- 无需 Docker 环境，任何 Go 开发环境均可运行
- 不影响现有代码逻辑，仅新增测试文件
- 仅覆盖 v2（sarama），v1（confluent-kafka-go）因 librdkafka C 库的 API 版本协商限制无法使用 MockBroker
