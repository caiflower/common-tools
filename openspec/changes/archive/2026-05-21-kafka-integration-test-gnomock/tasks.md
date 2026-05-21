## 1. 基础设施

- [x] 1.1 创建 `kafka/v2/producer_itest_test.go`，带 `//go:build integration` tag，包含 MockBroker 辅助函数（启动、配置 MockResponse 映射、关闭）

## 2. v2 Producer 集成测试（MockBroker）

- [x] 2.1 实现 v2 SyncProducer 发送测试：配置 MockBroker 的 MetadataResponse + ProduceResponse → SyncProducer.SendMessages → 验证无错误
- [x] 2.2 实现 v2 AsyncProducer 发送测试：配置 MockBroker 的 MetadataResponse + ProduceResponse → AsyncProducer.Input → 验证 Successes channel 收到确认

## 3. v2 Producer 幂等性测试（mocks 包）

- [x] 3.1 实现 v2 SyncProducer 幂等性测试：使用 `mocks.NewSyncProducer` 创建带 `Producer.Idempotent=true` 配置的 mock producer → 设置 Expectation → SendMessage → 验证消息被正确处理
- [x] 3.2 实现 v2 AsyncProducer 幂等性测试：使用 `mocks.NewAsyncProducer` 创建带 `Producer.Idempotent=true` 配置的 mock producer → 设置 Expectation → Input channel 发送 → 验证 Successes channel 收到确认

## 4. v2 Consumer 集成测试（MockBroker）

- [x] 4.1 实现 v2 ConsumerGroup 消费测试：配置 MockBroker 的 MetadataResponse + FindCoordinatorResponse + JoinGroupResponse + SyncGroupResponse + OffsetResponse + FetchResponse → ConsumerGroup.Consume → 验证消息被正确消费

## 5. 验证

- [x] 5.1 确认 `go test ./kafka/...` 不运行集成测试
- [x] 5.2 确认 `go test -tags=integration ./kafka/v2/...` 能运行集成测试
