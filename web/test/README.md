# Web Framework 测试套件

本目录包含Web框架的完整测试套件，包括单元测试、集成测试、性能基准测试等。

## 文件说明

- `server_test.go` - 基础服务器测试，包含Action和RESTful风格的基础测试用例
- `integration_test.go` - 集成测试套件，包含多个测试场景和基准测试
- `controller/v1/test/test.go` - 测试控制器，定义了各种请求参数和处理方法

## 运行测试

### 1. 运行所有测试

```bash
cd /Users/lijinlong/workspace/common-tools
go test ./web/test -v
```

### 2. 运行特定测试

```bash
# 运行RESTful POST请求测试
go test ./web/test -v -run TestRESTfulPostRequest

# 运行路径参数测试
go test ./web/test -v -run TestPathParameters

# 运行并发请求测试
go test ./web/test -v -run TestConcurrentRequests
```

### 3. 运行性能基准测试

```bash
# 运行所有基准测试，迭代1000次
go test ./web/test -bench=. -benchtime=1000x

# 运行特定基准测试
go test ./web/test -bench=BenchmarkRESTfulRequest -benchtime=1000x

# 生成内存分配统计
go test ./web/test -bench=. -benchmem
```

### 4. 运行测试并生成覆盖率报告

```bash
# 生成覆盖率
go test ./web/test -v -cover -coverprofile=coverage.out

# 查看覆盖率详情
go tool cover -html=coverage.out
```

### 5. 详细的测试命令示例

```bash
# 运行所有测试，显示详细日志
go test ./web/test -v -count=1

# 运行单个测试并显示日志输出
go test ./web/test -v -run TestRESTfulPostRequest -args -v

# 运行测试并显示内存分配
go test ./web/test -v -memprofilerate=1

# 运行测试并在超时前停止（30秒超时）
go test ./web/test -v -timeout=30s
```

## 测试用例说明

### 单元测试

#### `TestHttpServer`
- **描述**: 基础服务器启动和请求处理测试
- **场景**: 
  - RESTful POST请求
  - RESTful GET请求
  - 路径参数绑定
  - 错误响应验证
- **运行**: `go test ./web/test -v -run TestHttpServer`

### 集成测试

#### `TestRESTfulPostRequest`
- **描述**: 测试RESTful风格的POST请求
- **验证**: 请求成功、状态码200、响应数据结构正确

#### `TestRESTfulGetRequest`
- **描述**: 测试RESTful风格的GET请求
- **验证**: GET方法正确处理、返回正确的响应

#### `TestPathParameters`
- **描述**: 测试路径参数绑定功能
- **验证**: 路径参数 `{testId}` 和 `{subId}` 正确绑定到请求对象

#### `TestRequiredFieldValidation`
- **描述**: 测试必填字段验证
- **验证**: 缺少必填字段时返回400错误

#### `TestMultipleInterceptors`
- **描述**: 测试多个拦截器按顺序执行
- **验证**: 拦截器的Before/After方法按配置顺序执行

#### `TestRequestTraceID`
- **描述**: 测试请求追踪ID功能
- **验证**: 响应中包含RequestId，用于追踪

#### `TestHeaderBinding`
- **描述**: 测试HTTP请求头绑定
- **验证**: 自定义请求头正确绑定到请求对象的header标签字段

#### `TestErrorHandling`
- **描述**: 测试错误处理机制
- **验证**: 无效请求数据被正确处理

#### `TestConcurrentRequests`
- **描述**: 测试并发请求处理能力
- **验证**: 10个并发请求能正确处理，成功率不低于80%

### 性能基准测试

#### `BenchmarkRESTfulRequest`
- **描述**: RESTful请求性能基准测试
- **测量**: 请求处理速度、内存分配等
- **运行**: `go test ./web/test -bench=BenchmarkRESTfulRequest -benchtime=1000x`

## 测试场景覆盖

### 参数绑定
- ✓ JSON Body绑定
- ✓ 查询参数绑定
- ✓ 路径参数绑定
- ✓ 请求头绑定
- ✓ 默认值设置

### 参数校验
- ✓ 必填字段验证（verf标签）
- ✓ 枚举值验证（inList标签）
- ✓ 正则表达式验证（reg标签）
- ✓ 范围验证（between标签）
- ✓ 长度验证（len标签）

### 错误处理
- ✓ 参数验证失败返回400
- ✓ 业务异常处理
- ✓ 系统异常处理

### 拦截器
- ✓ 多个拦截器按顺序执行
- ✓ Before/After正确调用
- ✓ 异常处理（OnPanic）

### 并发处理
- ✓ 多个并发请求正确处理
- ✓ 请求隔离

## HTTP客户端使用示例

### 基本请求

```go
// 创建HTTP客户端
client := httpclient.NewHttpClient(httpclient.Config{
    Timeout: 20,
    Verbose: toBoolPtr(false),
})

// POST JSON请求
req := &UserRequest{Name: "John", Email: "john@example.com"}
var resp CommonResponse
httpResp := &httpclient.Response{Data: &resp}

err := client.PostJson(
    "request-id-001",
    "localhost:8080/api/users",
    req,
    httpResp,
    map[string]string{"Authorization": "Bearer token"},
)
```

### 不同HTTP方法

```go
// GET请求
client.Get(reqId, url, params, resp, headers)

// POST JSON
client.PostJson(reqId, url, body, resp, headers)

// POST Form
client.PostForm(reqId, url, formData, resp, headers)

// PUT请求
client.Put(reqId, url, body, resp, headers)

// PATCH请求
client.Patch(reqId, url, body, resp, headers)

// DELETE请求
client.Delete(reqId, url, body, resp, headers)

// 自定义请求
client.Do(method, reqId, url, contentType, body, values, resp, headers)
```

## 故障排除

### 端口冲突
如果测试运行失败显示"port already in use"：
1. 检查是否有其他进程占用端口（9090、9091、9092）
2. 使用 `lsof -i :9090` 查看占用进程
3. 修改测试中的Port配置

### 时区问题
测试会自动设置时区为Asia/Shanghai，如有问题检查系统时区配置。

### 网络问题
如果HTTP请求失败：
1. 确保服务器成功启动（查看日志输出）
2. 检查本地网络连接
3. 确保客户端和服务器在同一网络

## 扩展测试

要添加新的测试用例：

1. 在 `integration_test.go` 中添加新的测试函数：

```go
func TestNewFeature(t *testing.T) {
    suite := &TestSuite{}
    suite.setup(t)
    defer suite.teardown()
    
    // 编写测试逻辑
}
```

2. 运行测试：

```bash
go test ./web/test -v -run TestNewFeature
```

## CI/CD集成

在持续集成环境中运行测试：

```bash
# 运行所有测试并收集覆盖率
go test ./web/test -v -race -cover -coverprofile=coverage.out

# 检查覆盖率阈值（例如>80%）
go tool cover -func=coverage.out | grep total
```

## 性能调优

查看详细的性能数据：

```bash
# 显示内存分配统计
go test ./web/test -bench=. -benchmem

# CPU分析
go test ./web/test -bench=. -cpuprofile=cpu.prof
go tool pprof cpu.prof

# 内存分析
go test ./web/test -bench=. -memprofile=mem.prof
go tool pprof mem.prof
```

## 注意事项

1. **测试隔离**: 每个测试用例会启动独立的服务器实例，测试完成后会自动关闭
2. **时序问题**: 由于网络延迟，测试之间设置了适当的延迟（500ms）
3. **并发安全**: Web框架支持并发请求处理
4. **资源清理**: 使用defer确保所有资源正确释放

## 相关文档

- [Web框架使用指南](../../docs/web_usage_guide.md)
- [HTTP客户端文档](../../pkg/http)
