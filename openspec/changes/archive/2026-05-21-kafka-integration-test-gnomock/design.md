## Context

Kafka 模块提供 v1（confluent-kafka-go）和 v2（sarama）两套实现，包含 Producer 同步/异步发送和 Consumer 消费功能。当前测试仅覆盖单元级别，无法验证端到端的消息收发流程。新增的 `ProducerIdempotence` 配置也需要在更真实的环境下验证。

项目使用 Go modules，测试框架为 testify。sarama 自带 `MockBroker`（真实 TCP 服务器，实现 Kafka 协议）和 `mocks` 包（接口级 mock），无需 Docker 或任何外部依赖。

## Goals / Non-Goals

**Goals:**
- 使用 sarama 内置 MockBroker 验证 v2 Producer 的同步/异步发送
- 使用 sarama `mocks` 包验证 v2 Producer 的接口级行为
- 使用 sarama MockBroker 验证 v2 Consumer 的消息消费
- 验证 `ProducerIdempotence` 配置对 Producer 创建的影响（接口级 mock）
- 测试通过 build tag `integration` 隔离

**Non-Goals:**
- 不改造现有单元测试
- 不覆盖 v1（confluent-kafka-go）— librdkafka C 库的 API 版本协商与 MockBroker 不兼容
- 不验证 SASL/SSL 等安全认证场景
- 不验证 Consumer rebalance 等高级场景
- 不替换现有的测试框架

## Decisions

### 1. 使用 sarama MockBroker 而非 Docker 容器方案

**选择**: sarama 内置 MockBroker
**理由**:
- 零额外依赖，项目已依赖 sarama
- 无需 Docker，任何 Go 开发环境均可运行
- 真实 TCP 服务器，实现 Kafka 二进制协议，比 interface mock 更接近真实行为
- 启动毫秒级，无需等待容器拉起

**替代方案**:
- gnomock/testcontainers-go：需要 Docker，CI 环境受限
- kafka-mock-go：不支持 Produce API，无法测试 Producer

### 2. 两种 mock 策略组合使用

**选择**: MockBroker（协议级） + mocks 包（接口级）组合
**理由**:
- **MockBroker**：适合端到端 Producer→Consumer 流程验证，通过真实 TCP 连接测试完整协议交互
- **mocks 包**：适合验证 Producer 配置行为（如幂等性），直接 mock SyncProducer/AsyncProducer 接口

### 3. 集成测试使用 build tag `integration` 隔离

**选择**: `//go:build integration`
**理由**: MockBroker 测试虽然不需要 Docker，但仍属于集成测试范畴（测试多组件交互），应与单元测试隔离。
**替代方案**: 不用 build tag 直接放在 `_test.go` 中，但 MockBroker 测试启动 TCP 服务器，耗时比纯单元测试长。

### 4. 测试文件组织

**选择**:
- `kafka/v2/producer_itest_test.go` — v2 Producer 集成测试（MockBroker + mocks）
- `kafka/v2/consumer_itest_test.go` — v2 Consumer 集成测试（MockBroker）

**理由**: 与现有 `client_test.go` 命名风格一致，`itest` 前缀区分集成测试。

### 5. MockBroker 配置策略

**选择**: 使用 `SetHandlerByMap` 配置 MockResponse 映射
**理由**: sarama 提供了完整的 MockResponse 构建器（`NewMockMetadataResponse`、`NewMockProduceResponse`、`NewMockFetchResponse` 等），通过 map 映射请求类型到响应，清晰且可维护。

### 6. 幂等性测试策略

**选择**: 使用 `mocks` 包的接口级 mock 验证配置行为
**理由**: MockBroker 不支持 `InitProducerId` API（ApiKey 22），开启幂等性的 Producer 创建会因协议协商失败而报错。改用 `mocks.NewSyncProducer` / `mocks.NewAsyncProducer` 验证配置传递和消息发送行为。

## Risks / Trade-offs

- **[v1 无法覆盖]** → confluent-kafka-go 使用 librdkafka C 库，与 MockBroker 协议不兼容。v1 的集成测试仍需 Docker 方案，不在本次范围内
- **[MockBroker 协议简化]** → MockBroker 不支持所有 Kafka API（如 InitProducerId、CreateTopics），部分高级功能无法通过 MockBroker 测试
- **[MockResponse 需手动配置]** → 每个测试用例需要手动配置 MockResponse 映射，比真实 Kafka 更繁琐，但更可控
