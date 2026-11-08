# Kubernetes 部署与扩缩容

`cluster` 通过 `ReplicasDiscovery` 自动发现 StatefulSet 副本，无需在配置中手写
`Nodes`。节点列表变化时集群会自动重新加载。

> 适用模式：`cluster`（标准集群）。Redis 模式的 Kubernetes 配置见
> [redis-discovery.md](redis-discovery.md)。

## 部署要求

1. 使用 **StatefulSet** 部署，Pod 名称带稳定序号；
2. 创建 **Headless Service**，保证每个 Pod 有稳定的 DNS 记录；
3. Pod 域名格式符合：
   `<service>-<序号>.<namespace>.svc.cluster.local`；
4. 所有副本使用相同的通信端口（`ReplicasDiscovery.Port`）。

## 配置

```go
config := cluster.Config{
    Mode:    "cluster",
    Timeout: 10 * time.Second,
    Enable:  "true",
    ReplicasDiscovery: cluster.ReplicasDiscovery{
        DomainPatten: "my-service-{suf}.my-namespace.svc.cluster.local",
        Port:         8080,
        PollInterval: 30 * time.Second,
    },
}
```

| 字段 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `DomainPatten` | `string` | - | 域名模式，`{suf}` 会替换为副本序号 |
| `Port` | `int` | 8081 | 节点通信端口 |
| `PollInterval` | `time.Duration` | 30s | 轮询 headless service DNS 的间隔 |

> [!IMPORTANT]
> 字段名是 `DomainPatten`（历史拼写），不是 `DomainPattern`。
> 旧版本文档中的 `Replicas` 字段在当前版本不存在，副本数由 DNS 解析结果决定。

## 完整示例

```go
package main

import (
    "fmt"
    "time"

    "github.com/caiflower/common-tools/cluster"
)

func main() {
    config := cluster.Config{
        Mode:    "cluster",
        Timeout: 10 * time.Second,
        Enable:  "true",
        ReplicasDiscovery: cluster.ReplicasDiscovery{
            DomainPatten: "my-service-{suf}.my-namespace.svc.cluster.local",
            Port:         8080,
            PollInterval: 30 * time.Second,
        },
    }

    c, err := cluster.NewCluster(config)
    if err != nil {
        panic(err)
    }

    if err := c.Start(); err != nil {
        panic(err)
    }
    defer c.Close()

    for !c.IsReady() {
        time.Sleep(time.Second)
    }

    fmt.Printf("集群就绪，主节点: %s，节点数: %d\n",
        c.GetLeaderName(), c.GetAllNodeCount())
}
```

## 扩容

1. 增加 StatefulSet 副本数；
2. 新 Pod 启动后向集群注册；
3. 现有节点在下一次 DNS 轮询或节点同步时发现新节点；
4. 新节点可参与主节点选举。

无需修改现有 Pod 的配置，也无需重启。

## 缩容

1. 减少 StatefulSet 副本数；
2. 被删除的 Pod 在关闭时通知其他节点重新加载；
3. 其他节点在下一轮同步中移除该节点；
4. 如果被删除的是主节点，会触发重新选举。

> [!TIP]
> 缩容时建议先观察新节点稳定，再逐步减少副本；避免一次性删除多个节点导致在线节点少于半数。

## 节点识别

框架会自动调用 `global/env` 的 `GetLocalDNS()` 和 `GetLocalHostIP()` 识别当前节点，
通常无需手动设置 `Config.Nodes[].Local`。

如果 Pod 网络或 DNS 配置特殊，无法自动识别，可以显式设置 `Local: true` 或通过
`LOCAL_HOST_IP` 环境变量注入 Pod IP：

```yaml
env:
  - name: LOCAL_HOST_IP
    valueFrom:
      fieldRef:
        fieldPath: status.podIP
  - name: POD_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
```

## 手动重新加载

需要立即刷新节点列表时，可以调用内置远程函数：

```go
_, err := cluster.CallFuncAs[any](
    c,
    cluster.NewFuncSpec("node2", "cluster.ReloadAllNodes", 3, 2*time.Second),
)
```

## 排查清单

| 现象 | 检查项 |
| --- | --- |
| 新 Pod 未加入集群 | Headless Service 是否存在；DNS 是否可解析；`DomainPatten` 是否正确 |
| 节点数不正确 | `PollInterval` 是否过长；StatefulSet 副本数是否与预期一致 |
| 选主失败 | 在线节点是否少于半数；节点间端口是否互通 |
| 本节点识别错误 | `LOCAL_HOST_IP`、`POD_NAMESPACE` 是否正确注入 |

## 相关文档

- [Cluster 集群管理](README.md)
- [Redis 模式动态节点发现](redis-discovery.md)
- 源码：[cluster/cluster.go](../../../cluster/cluster.go)
