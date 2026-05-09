# common-tools

一个功能强大的 Go 语言工具库集合，提供了丰富的基础设施组件和工具类。

## 📦 安装

```bash
go get github.com/caiflower/common-tools
```
## 📚 核心组件

### 🏗️ 基础设施

| 组件 | 路径 | 描述 | 文档                           |
| --- | --- | --- |------------------------------|
| **依赖注入（IOC）** | `github.com/caiflower/common-tools/pkg/bean` | 自动注入和管理单例，类似Java的IOC容器 | [📖 详细文档](./docs/bean.md)    |
| **cluster（集群管理）** | `github.com/caiflower/common-tools/cluster` | 基于Raft算法实现的集群管理，支持集群master选举和远程调用 | [📖 详细文档](./docs/cluster.md) |
| **web框架** | `github.com/caiflower/common-tools/web/v1` | 轻量级web框架，支持tag参数校验、interceptor过滤和RESTful接口 | [📖 详细文档](./docs/web.md)     |
| **global（全局管理）** | `github.com/caiflower/common-tools/global` | 全局资源管理器，管理Daemon进程，实现程序优雅退出 |                              |
| **taskx（任务框架）** | `github.com/caiflower/common-tools/taskx` | 任务调度框架，支持集群调度、DAG流任务、子任务结果传递（依赖MySQL） | [📖 详细文档](./docs/taskx/README.md)     |

### 🗄️ 数据库与缓存客户端

| 组件 | 路径 | 描述 | 文档 |
| --- | --- | --- | --- |
| **redis-client** | `github.com/caiflower/common-tools/redis/v1` | Redis客户端封装 | |
| **db-client** | `github.com/caiflower/common-tools/db/v1` | 数据库连接客户端，基于bun实现，支持MySQL/PostgreSQL/Oracle等 | |
| **clickhouse-client** | `github.com/caiflower/common-tools/ck/v1` | ClickHouse客户端封装 | |
| **kafka-client**      | `github.com/caiflower/common-tools/kafka/` | 分为v1和v2版本，v1依赖cgo，v2不依赖 | |

### 🧰 工具包（pkg）

| 组件 | 路径 | 描述 | 文档 |
| --- | --- | --- | --- |
| **basic（数据结构）** | `github.com/caiflower/common-tools/pkg/basic` | Set、LinkedList、LinkedHashMap、PriorityQueue、DelayQueue等 | |
| **cache（缓存）** | `github.com/caiflower/common-tools/pkg/cache` | LocalCache、LRU、LFU等本地缓存实现 | |
| **golocal** | `github.com/caiflower/common-tools/pkg/golocal/v1` | 协程本地存储，类似Java的ThreadLocal | |
| **limiter（限流器）** | `github.com/caiflower/common-tools/pkg/limiter` | 固定窗口和令牌桶限流算法实现 | |
| **logger（日志）** | `github.com/caiflower/common-tools/pkg/logger` | 日志框架，支持标准输出和文件输出 | |
| **syncx（自旋锁）** | `github.com/caiflower/common-tools/pkg/syncx` | 自旋锁实现（来自ants项目） | |
| **crontab（定时任务）** | `github.com/caiflower/common-tools/pkg/crontab` | 基于cron表达式的定时任务框架 | |
| **office** | `github.com/caiflower/common-tools/pkg/office` | Excel文件处理工具 | |
| **tools（工具类）** | `github.com/caiflower/common-tools/pkg/tools` | 常用工具函数集合（JSON、加密、文件等） | |

## 🚀 快速开始

https://github.com/caiflower/cf-tools 快速创建项目

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 License

请查看项目根目录的 LICENSE 文件

