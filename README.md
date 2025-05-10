# 如何使用？

https://github.com/caiflower/cf-tools

# common-tools

| 名称              | 路径                                             | 描述                                                         |
| ----------------- | ------------------------------------------------ | ------------------------------------------------------------ |
| syncx（自旋锁）   | github.com/caiflower/common-tools/pkg/syncx      | 自旋锁，拷贝自ants项目                                       |
| 依赖注入（IOC）   | github.com/caiflower/common-tools/pkg/bean       | 自动注入和管理单例，类似Java的IOC                            |
| cluster           | github.com/caiflower/common-tools/cluster        | 基于raft算法实现的集群管理，支持集群master运行，集群远程调用 |
| web框架           | github.com/caiflower/common-tools/web/v1         | 轻量web框架，支持tag参数校验、interceptor过滤和restful接口等 |
| global            | github.com/caiflower/common-tools/global         | 资源全局管理器，管理全局Daemon进程，实现程序优雅退出         |
| redis-client      | github.com/caiflower/common-tools/redis/v1       | 封装redis的client                                            |
| db-client         | github.com/caiflower/common-tools/db/v1          | 数据库连接client，基于bun实现，支持mysq/pg/oracle等数据库，目前仅适配mysql |
| clickhouse-client | github.com/caiflower/common-tools/ck/v1          | 封装ck的client                                               |
| taskx             | github.com/caiflower/common-tools/taskx          | 任务框架，支持集群调度、DAG流任务调度、子任务结果传递等。依赖mysql |
| basic             | github.com/caiflower/common-tools/pkg/basic      | Set、LinkedList、LinkedHashMap、PriorityQueue和延迟队列等数据结构 |
| cache             | github.com/caiflower/common-tools/pkg/cache      | LocalCache、LRU和LFU等本地缓存                               |
| golocal           | github.com/caiflower/common-tools/pkg/golocal/v1 | 类似Java的ThreadLocal                                        |
| limiter           | github.com/caiflower/common-tools/pkg/limiter    | 固定窗口和令桶牌限流算法实现，web框架能通过配置文件设置使用  |
| logger            | github.com/caiflower/common-tools/pkg/logger     | 日志框架，支持标准输出和文件输出。                           |
| tools             | github.com/caiflower/common-tools/pkg/tools      | 工具类                                                       |
| office            | github.com/caiflower/common-tools/pkg/office     | excel                                                        |
| crontab           | github.com/caiflower/common-tools/pkg/crontab    | 集运cron实现的定时任务框架                                   |

