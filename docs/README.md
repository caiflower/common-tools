# common-tools 文档

本目录是 common-tools 的文档入口。项目概览、安装和快速开始见根 [README](../README.md)。

## 文档导航

### 基础设施

| 组件 | 导入路径 | 文档 |
| --- | --- | --- |
| Bean 依赖注入 | `github.com/caiflower/common-tools/pkg/bean` | [bean.md](infrastructure/bean.md) |
| Web 框架 | `github.com/caiflower/common-tools/web` | [web/README.md](infrastructure/web/README.md) |
| Cluster 集群管理 | `github.com/caiflower/common-tools/cluster` | [cluster/README.md](infrastructure/cluster/README.md) |

Web 子文档：

- [服务端](infrastructure/web/server.md)
- [HTTP 客户端](infrastructure/web/client.md)
- [命令行双模式](infrastructure/web/cli.md)

Cluster 子文档：

- [Redis 模式动态节点发现](infrastructure/cluster/redis-discovery.md)
- [Kubernetes 部署与扩缩容](infrastructure/cluster/kubernetes.md)

### 数据库与缓存客户端（待补充）

| 组件 | 导入路径 |
| --- | --- |
| Redis 客户端 v1 | `github.com/caiflower/common-tools/redis/v1` |
| Redis 客户端 v2 | `github.com/caiflower/common-tools/redis/v2` |
| 数据库客户端 | `github.com/caiflower/common-tools/db/v1` |
| ClickHouse 客户端 | `github.com/caiflower/common-tools/ck/v1` |
| Kafka 客户端 v1（cgo） | `github.com/caiflower/common-tools/kafka/v1` |
| Kafka 客户端 v2（pure Go） | `github.com/caiflower/common-tools/kafka/v2` |

### 可观测性与工具包（待补充）

| 组件 | 导入路径 |
| --- | --- |
| 全局资源管理 | `github.com/caiflower/common-tools/global` |
| 配置加载 | `github.com/caiflower/common-tools/global/config` |
| 环境变量 | `github.com/caiflower/common-tools/global/env` |
| OpenTelemetry | `github.com/caiflower/common-tools/otel` |

工具包后续按「每包一篇」拆分：`basic`、`cache`、`limiter`、`logger`、`crontab`、
`golocal`、`office`、`tools`、`http`、`json`、`nio`、`pool`、`safego`、`shell`、
`syncx`、`inflight`、`bytebufferpool`。

## 阅读路径

| 目标 | 推荐顺序 |
| --- | --- |
| 第一次使用 | 根 [README](../README.md) → [web/README.md](infrastructure/web/README.md) → [web/server.md](infrastructure/web/server.md) |
| 开发 HTTP 服务 | [bean.md](infrastructure/bean.md) → [web/server.md](infrastructure/web/server.md) → [web/client.md](infrastructure/web/client.md) |
| 做主从/高可用 | [cluster/README.md](infrastructure/cluster/README.md) → [kubernetes.md](infrastructure/cluster/kubernetes.md) 或 [redis-discovery.md](infrastructure/cluster/redis-discovery.md) |
| 接入命令行 | [web/cli.md](infrastructure/web/cli.md) |

## 文档规范

新增或修改文档时遵循以下约定：

1. **语言与路径**：正文使用中文，文件名使用小写英文和连字符。
2. **统一结构**：概述 → 特性/适用场景 → 快速开始 → 核心 API → 配置 → 进阶 → 最佳实践 → FAQ → 相关文档。
3. **代码示例**：快速开始和完整示例给出完整 import；片段示例可以省略样板代码。
4. **配置表**：列出名称、类型、默认值和说明；区分 Option 函数与仅结构体字段。
5. **版本路径**：注明实际 import 路径，避免使用不存在的版本后缀。
6. **相对链接**：文档之间使用相对路径互相引用，提交前检查链接有效性。
7. **提示块**：使用 GitHub 支持的 `> [!NOTE]`、`> [!TIP]`、`> [!WARNING]`、`> [!IMPORTANT]`。

## 资产

| 资产 | 说明 |
| --- | --- |
| `assets/images/cluster_architecture.png` | Cluster 架构图 |
| `assets/grafana/app-monitor.json` | 服务监控 Grafana 面板（Web / DB / Redis / 进程 / Cluster） |
| `assets/grafana/README.md` | 面板说明与导入步骤 |

## 贡献

发现文档错误或有补充建议，欢迎提交 Issue 或 Pull Request。修改文档时请同时检查：

- 代码示例能否通过编译；
- API 名称、字段名和默认值是否与当前分支一致；
- 文档间的相对链接是否有效。
