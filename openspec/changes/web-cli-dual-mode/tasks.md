## 1. 路由元数据基础

- [x] 1.1 在 `web/router` 定义导出的 `RouteInfo`/`ParamInfo` 类型，包含 method/path/operationID/resource/verb/params 与参数来源、类型、校验信息
- [x] 1.2 为 `Handler` 增加线程安全的元数据注册表，并在 `addRoute` 时从最后一个 handler 提取 operationID、参数结构并写入
- [x] 1.3 实现 resource/verb 推导：方法名优先、path 兜底、HTTP method 映射，冲突时生成 `call <operationID>` 兜底信息
- [x] 1.4 为 `Handler` 增加并发安全的 `Routes()` 只读枚举 API
- [x] 1.5 增加显式覆盖 API，允许为指定路由设置 resource/verb
- [x] 1.6 为元数据注册与推导补齐单元测试（普通函数、结构体方法、gRPC、匿名 HandlerFunc、显式覆盖）

## 2. CLI 发现接口

- [x] 2.1 在 `web/app/server/config` 增加 CLI 元数据开关与路径配置项（默认关闭）
- [x] 2.2 在 `specialRequest` 中实现 `GET /cli/routes`，返回 name/version/resources/routes JSON
- [x] 2.3 实现 ETag/`If-None-Match` 支持，匹配时返回 304
- [x] 2.4 补充 `/cli/routes` 开启/关闭、内容正确性、304 条件请求测试

## 3. CLI 包本地模式

- [x] 3.1 引入 cobra 依赖，创建 `web/cli` 包和 root command，定义最小 `ResourceManager` 接口
- [x] 3.2 实现 `serve` 子命令：将 engine 注册为 daemon 并调用 `Signal()`，默认使用 `global.DefaultResourceManger`，支持自定义管理器 option
- [x] 3.3 从 `engine.Routes()` 生成本地动态命令树（`动词 + 资源` + `call <operationID>` 兜底）
- [x] 3.4 为 path/query/header 参数生成 flag，`--data`/`-f` 处理 body，GET/HEAD 的 json 字段映射为 query
- [x] 3.5 使用 `web/app/client` 发起 HTTP 请求，支持 `--server` 默认地址解析、`--token`、`--header`
- [x] 3.6 实现 `table/json/yaml` 输出
- [x] 3.7 补充本地模式命令生成、请求构造、输出格式、资源管理器集成测试

## 4. 远程发现与缓存

- [x] 4.1 实现远端 `/cli/routes` 拉取与元数据反序列化
- [x] 4.2 实现缓存文件读写、TTL（默认 5 分钟）、server 地址 hash 命名
- [x] 4.3 实现 `If-None-Match` 条件刷新，304 时保留缓存并更新时间戳
- [x] 4.4 实现 `routes` 列表命令与 `--refresh` 强制刷新，网络失败有缓存时降级并警告
- [x] 4.5 补充远程发现、缓存命中、强制刷新、304、降级场景测试

## 5. 文档与收尾

- [x] 5.1 更新 `docs/web.md`：双模式用法、命令示例、`/cli/routes` 配置与安全说明
- [x] 5.2 运行 `go test ./web/...` 和相关静态检查，确认无回归
