# common-tools

一个面向 Go 微服务的工具库集合，提供依赖注入、Web 框架、集群管理、数据客户端和常用工具包。

## 安装

```bash
go get github.com/caiflower/common-tools
```

## 文档

完整文档见 [docs/README.md](./docs/README.md)，包含组件导航、阅读路径和文档规范。

## 核心组件

### 基础设施

| 组件 | 导入路径 | 描述 | 文档 |
| --- | --- | --- | --- |
| **依赖注入（IoC）** | `github.com/caiflower/common-tools/pkg/bean` | 自动注入和管理单例，类似 Spring 的 IoC 容器 | [详细文档](./docs/infrastructure/bean.md) |
| **Web 框架** | `github.com/caiflower/common-tools/web` | 高性能 RESTful 框架，支持参数校验、中间件、gRPC 和 CLI 双模式 | [详细文档](./docs/infrastructure/web/README.md) |
| **Cluster 集群管理** | `github.com/caiflower/common-tools/cluster` | 基于 Raft 的集群管理，支持主节点选举、远程调用和任务调度 | [详细文档](./docs/infrastructure/cluster/README.md) |
| **Global 全局管理** | `github.com/caiflower/common-tools/global` | 全局资源管理器，管理 Daemon 进程，实现优雅退出 | 待补充 |
| **OpenTelemetry** | `github.com/caiflower/common-tools/otel` | Web、gRPC、Kafka、Redis、DB 的链路追踪接入 | 待补充 |

### 数据库与缓存客户端

| 组件 | 导入路径 | 描述 | 文档 |
| --- | --- | --- | --- |
| **Redis 客户端** | `github.com/caiflower/common-tools/redis/v1`<br>`github.com/caiflower/common-tools/redis/v2` | Redis 客户端封装，v1 基于 go-redis v8，v2 基于 v9 | 待补充 |
| **数据库客户端** | `github.com/caiflower/common-tools/db/v1` | 基于 bun 的数据库客户端，支持 MySQL、PostgreSQL、SQLite | 待补充 |
| **ClickHouse 客户端** | `github.com/caiflower/common-tools/ck/v1` | ClickHouse 客户端封装 | 待补充 |
| **Kafka 客户端** | `github.com/caiflower/common-tools/kafka/v1`<br>`github.com/caiflower/common-tools/kafka/v2` | v1 基于 confluent-kafka-go（cgo），v2 基于 Sarama（纯 Go） | 待补充 |

### 工具包

| 组件 | 导入路径 | 描述 | 文档 |
| --- | --- | --- | --- |
| **basic（数据结构）** | `github.com/caiflower/common-tools/pkg/basic` | Set、LinkedList、LinkedHashMap、PriorityQueue、DelayQueue 等 | 待补充 |
| **cache（缓存）** | `github.com/caiflower/common-tools/pkg/cache` | LocalCache、LRU、LFU 等本地缓存实现 | 待补充 |
| **golocal（协程本地存储）** | `github.com/caiflower/common-tools/pkg/golocal/v1` | 类似 Java ThreadLocal 的协程本地存储 | 待补充 |
| **limiter（限流器）** | `github.com/caiflower/common-tools/pkg/limiter` | 固定窗口、令牌桶、Redis 限流等实现 | 待补充 |
| **logger（日志）** | `github.com/caiflower/common-tools/pkg/logger` | 支持标准输出和文件输出的日志框架 | 待补充 |
| **syncx（自旋锁）** | `github.com/caiflower/common-tools/pkg/syncx` | 自旋锁实现 | 待补充 |
| **crontab（定时任务）** | `github.com/caiflower/common-tools/pkg/crontab` | 基于 cron 表达式的定时任务框架 | 待补充 |
| **office（Excel）** | `github.com/caiflower/common-tools/pkg/office` | Excel 文件处理工具 | 待补充 |
| **tools（工具函数）** | `github.com/caiflower/common-tools/pkg/tools` | JSON、加密、文件等常用工具函数 | 待补充 |

## 快速开始

推荐使用 [cf-tools](https://github.com/caiflower/cf-tools) 创建项目骨架，再按
[Web 快速开始](./docs/infrastructure/web/README.md#快速开始) 接入框架。

## 贡献

欢迎提交 Issue 和 Pull Request。修改文档前请阅读 [文档规范](./docs/README.md#文档规范)。

## License

项目使用 Apache License 2.0，详见根目录 [LICENSE](./LICENSE)。
