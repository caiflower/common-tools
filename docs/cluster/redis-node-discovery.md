# Redis 模式动态节点发现配置说明

## 概述

Redis 模式支持基于 Redis 的动态节点发现机制，节点信息不再依赖配置文件，而是自动从 Redis 中获取。当集群扩容或缩容时，节点能够自动感知变化。

## 工作原理

### 1. 节点注册
- 每个节点启动时，向 Redis 注册自己的信息（节点名称、地址、时间戳）
- 注册信息带有 TTL（默认 60 秒），防止僵尸节点

### 2. 心跳续约
- 节点定期（默认 20 秒）续约自己的注册信息
- 如果节点宕机，注册信息会自动过期

### 3. 节点同步
- 每个节点定期（默认 30 秒）从 Redis 扫描所有活跃节点
- 自动发现新加入的节点
- 自动移除已下线的节点

### 4. Redis Key 结构
```
{DataPath}:Nodes:{NodeName}  ->  JSON 格式的节点信息
{DataPath}:Election          ->  当前 Leader 名称
```

## 配置示例

### YAML 配置

```yaml
cluster:
  mode: redis                    # 使用 Redis 模式
  enable: "true"
  timeout: 10s
  
  # Redis 发现配置
  redisDiscovery:
    beanName: ""                 # Redis Bean 名称（可选，为空则使用默认）
    dataPath: "myapp:cluster"    # Redis Key 前缀
    port: 8081                   # 节点通信端口
    electionInterval: 15s        # 选主/续约间隔
    electionPeriod: 30s          # 选主/续约有效租期
    syncLeaderInterval: 10s      # 同步 Leader 间隔
    nodeRegisterTTL: 60s         # 节点注册信息过期时间
    nodeSyncInterval: 30s        # 节点信息同步间隔
    nodeHeartbeatPeriod: 20s     # 节点心跳续约周期
```

### 配置参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `dataPath` | - | Redis Key 前缀，用于隔离不同应用的集群数据 |
| `port` | 8081 | **新增**：节点通信端口，所有节点使用相同端口 |
| `electionInterval` | 15s | Leader 续约检查间隔 |
| `electionPeriod` | 30s | Leader 租期时间 |
| `syncLeaderInterval` | 10s | 同步 Leader 信息的间隔 |
| `nodeRegisterTTL` | 60s | 节点注册信息的过期时间，建议设置为 `nodeHeartbeatPeriod` 的 3 倍 |
| `nodeSyncInterval` | 30s | 从 Redis 同步节点列表的间隔 |
| `nodeHeartbeatPeriod` | 20s | 节点心跳续约周期，应小于 `nodeRegisterTTL` |

## 扩容/缩容流程

### 扩容（添加节点）

1. **启动新节点**
   ```bash
   # 新节点启动时会自动：
   # 1. 向 Redis 注册自己
   # 2. 开始心跳续约
   # 3. 从 Redis 同步其他节点信息
   ```

2. **现有节点自动发现**
   - 其他节点在下次同步周期（默认 30 秒）内会发现新节点
   - 自动建立连接

3. **参与选举**
   - 新节点可以参与 Leader 选举
   - 如果成为 Leader，会接管集群调度

### 缩容（移除节点）

1. **正常关闭节点**
   ```bash
   # 节点关闭时会自动：
   # 1. 从 Redis 删除自己的注册信息
   # 2. 释放 Leader 身份（如果是 Leader）
   ```

2. **其他节点自动感知**
   - 其他节点在下次同步时会发现该节点已消失
   - 自动从节点列表中移除

3. **异常宕机**
   - 如果节点异常宕机，注册信息会在 TTL 过期后自动删除
   - 其他节点同步时会自动移除该节点
   - 如果宕机的是 Leader，会触发重新选举

## 监控建议

### Redis Key 监控

```bash
# 查看所有注册的节点
redis-cli KEYS "myapp:cluster:Nodes:*"

# 查看某个节点的详细信息
redis-cli GET "myapp:cluster:Nodes:node1"

# 查看当前 Leader
redis-cli GET "myapp:cluster:Election"

# 查看 Key 的剩余 TTL
redis-cli TTL "myapp:cluster:Nodes:node1"
```

### 日志监控

关注以下日志关键字：
- `[cluster-redis] new node discovered` - 发现新节点
- `[cluster-redis] node removed` - 节点移除
- `[cluster-redis] register node failed` - 节点注册失败
- `[cluster-redis] sync nodes failed` - 节点同步失败

## 注意事项

1. **时间同步**：确保所有节点的时间同步，建议使用 NTP
2. **Redis 可用性**：Redis 是单点故障点，建议使用 Redis Sentinel 或 Redis Cluster
3. **网络分区**：网络分区可能导致节点误判，合理配置 TTL 和同步间隔
4. **节点命名**：建议使用唯一的节点名称（如 hostname、pod name）
5. **端口配置**：所有节点使用 `redisDiscovery.port` 配置的相同端口，无需为每个节点单独配置

## 故障排查

### 节点无法被发现

1. 检查 Redis 连接是否正常
2. 检查 `dataPath` 配置是否一致
3. 查看节点日志是否有注册失败的错误
4. 使用 `redis-cli` 检查 Key 是否存在

### 节点未及时移除

1. 检查 `nodeRegisterTTL` 配置是否合理
2. 确认节点是否正常关闭（正常关闭会删除注册信息）
3. 检查网络是否导致心跳续约失败

### Leader 选举异常

1. 检查所有节点的 `dataPath` 是否一致
2. 确认 Redis 连接正常
3. 查看选举相关日志

## 与配置文件的区别

| 特性 | 配置文件模式 | Redis 动态发现模式 |
|------|-------------|-------------------|
| 节点配置 | 静态，修改需重启 | 动态，自动感知 |
| 扩容 | 需修改所有节点配置并重启 | 启动新节点即可 |
| 缩容 | 需修改所有节点配置并重启 | 关闭节点即可 |
| 适用场景 | 固定规模集群 | 弹性伸缩、云原生环境 |
| 依赖性 | 无 | 依赖 Redis 可用性 |

## 最佳实践

1. **生产环境**：使用 Redis Sentinel 或 Redis Cluster 保证高可用
2. **TTL 配置**：`nodeRegisterTTL` >= 3 * `nodeHeartbeatPeriod`
3. **同步间隔**：`nodeSyncInterval` 根据集群规模调整，大规模集群可适当增大
4. **监控告警**：监控 Redis 连接状态和节点数量变化
5. **灰度发布**：扩容时先启动新节点，观察稳定后再缩容旧节点
