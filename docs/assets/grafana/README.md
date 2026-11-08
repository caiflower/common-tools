# Grafana 监控面板

`app-monitor.json` 是 common-tools 的 Grafana 面板（面板标题为 `Server Monitor`），
覆盖 Web、数据库、Redis、进程和集群指标。

## 面板内容

| 分组 | 面板 | 指标 |
| --- | --- | --- |
| SLI | P95、QPS、请求频率 | `http_request_duration_seconds_bucket`、`http_request_duration_seconds_count` |
| SLO | P95、QPS | `http_request_duration_seconds_bucket`、`http_request_total` |
| SLO | HTTP 状态码分布 | `http_request_total{status}` |
| Performance | 内存、CPU、文件描述符、启动时长 | `process_resident_memory_bytes`、`process_cpu_seconds_total`、`process_open_fds`、`process_start_time_seconds` |
| Performance | Cluster leader | `cluster_member_is_leader` |
| Database | Query P95、QPS、错误率、连接池 | `db_query_duration_seconds_bucket`、`db_query_total`、`db_stats_*` |
| Redis | Command P95、QPS、错误率、连接池 | `redis_command_duration_seconds_bucket`、`redis_commands_total`、`redis_pool_*` |

## 前置条件

1. 服务暴露 Prometheus 指标端点：

   ```go
   server := web.Default(
       config.WithMetrics(true),
   )
   ```

   默认端点为 `GET /metrics`。

2. 数据库和 Redis 指标由客户端包自动注册，只要同一个 `/metrics` 端点被 Prometheus 抓取即可。

3. 面板使用 `namespace` 和 `app` 两个变量：
   - 在 Kubernetes 中由 `POD_NAMESPACE`、`POD_APP` 环境变量注入；
   - 未设置时指标不带这两个标签，变量选择 `All` 即可（查询使用 `=~".*"`）。

## 导入步骤

1. 打开 Grafana，进入 **Dashboards → New → Import**；
2. 上传 `app-monitor.json`；
3. 在数据源提示中选择 Prometheus（对应 JSON 中的 `DS_PROMETHEUS` 输入）；
4. 选择 `namespace` 和 `app`，保存面板。

## 变量

| 变量 | 来源 | 说明 |
| --- | --- | --- |
| `namespace` | `label_values(namespace)` | Kubernetes 命名空间 |
| `app` | `label_values({namespace=~"$namespace"}, app)` | 应用名 |

两个变量都支持 `All`，默认选择全部。

## 指标说明

| 指标 | 类型 | 标签 |
| --- | --- | --- |
| `http_request_total` | Counter | `web`、`status`、`method`、`handler`、`ip`，可选 `namespace`、`app` |
| `http_request_duration_seconds` | Histogram | `handler`、`ip`，可选 `namespace`、`app` |
| `db_query_total` | Counter | `dialect`、`database`、`status`，可选 `namespace`、`app` |
| `db_query_duration_seconds` | Histogram | `dialect`、`database`，可选 `namespace`、`app` |
| `db_stats_*` | Gauge / Counter | `dialect`、`database`，可选 `namespace`、`app` |
| `redis_commands_total` | Counter | `addr`、`command`、`status`，可选 `namespace`、`app` |
| `redis_command_duration_seconds` | Histogram | `addr`、`command`，可选 `namespace`、`app` |
| `redis_pool_*` | Gauge / Counter | `addr`，可选 `namespace`、`app` |
| `cluster_member_is_leader` | Gauge | `node`、`ip`，可选 `namespace`、`app` |

> [!NOTE]
> 面板中的 `http_requests_total` 已修正为当前代码实际输出的 `http_request_total`。
> 如果从旧版本升级，请同步更新已有面板中的指标名。

## 相关文档

- [Web 服务端 - Prometheus 指标](../../infrastructure/web/server.md#prometheus-指标)
- [Cluster 集群管理](../../infrastructure/cluster/README.md)
- [文档索引](../../README.md)
