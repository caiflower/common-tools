## 1. ICluster 接口扩展

- [x] 1.1 在 `ICluster` 接口中新增 `RegisterGRPCService(sd *grpc.ServiceDesc, ss interface{}) error` 方法签名
- [x] 1.2 在 `ICluster` 接口中新增 `GetGRPCClient(nodeName string) (grpc.ClientConnInterface, error)` 方法签名
- [x] 1.3 在 `Cluster` 结构体中新增 `registeredServices map[string]*grpc.ServiceDesc` 字段，用于存储预注册的服务和检测重复注册
- [x] 1.4 在 `Cluster` 结构体中新增 `registeredImpls map[string]interface{}` 字段，用于存储预注册的服务实现

## 2. RegisterGRPCService 实现

- [x] 2.1 实现 `RegisterGRPCService` 方法：检查服务名是否重复，重复则返回 error
- [x] 2.2 处理集群启动前注册：将 `sd` 和 `ss` 存入 `registeredServices` / `registeredImpls`，在 `listen()` 中随 `ClusterService` 一起注册
- [x] 2.3 处理集群运行时注册：返回错误（gRPC 不支持 Server.Serve 后动态注册服务）
- [x] 2.4 在 `listen()` 方法中遍历 `registeredServices`，将预注册的服务注册到 gRPC server

## 3. GetGRPCClient 实现

- [x] 3.1 实现 `GetGRPCClient` 方法：从 `aliveNodes` 中查找节点，返回其 `grpc.ClientConnInterface`
- [x] 3.2 处理节点不存在和不可达的情况，返回明确错误信息
- [x] 3.3 在 `Node` 结构体中暴露 `grpcConn` 的 `ClientConnInterface`（通过新增方法或直接访问）

## 4. 测试

- [x] 4.1 编写单元测试：启动前注册 gRPC 服务，验证客户端可调用
- [x] 4.2 编写单元测试：运行时动态注册 gRPC 服务，验证返回错误
- [x] 4.3 编写单元测试：重复注册同名的 gRPC 服务，验证返回错误
- [x] 4.4 编写单元测试：`GetGRPCClient` 获取存活节点连接，创建原生 stub 并调用
- [x] 4.5 编写单元测试：`GetGRPCClient` 对不存在/不可达节点返回错误
- [x] 4.6 编写 benchmark 测试：对比原生 gRPC 调用与 `CallFuncAs` 的性能差异
