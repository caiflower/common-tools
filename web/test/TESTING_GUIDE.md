# Web框架测试快速指南

## 概述

本测试套件包含完整的单元测试和集成测试，使用HTTP客户端进行端到端测试。

## 快速开始

### 运行所有测试
```bash
cd /Users/lijinlong/workspace/common-tools
go test ./web/test -v
```

### 运行特定测试
```bash
# RESTful POST请求
go test ./web/test -v -run TestRESTfulPostRequest

# 路径参数测试
go test ./web/test -v -run TestPathParameters

# 并发请求测试
go test ./web/test -v -run TestConcurrentRequests
```

## 可用的测试用例

### 基础功能测试

| 测试名称 | 描述 | 命令 |
|---------|------|------|
| `TestRESTfulPostRequest` | RESTful POST请求 | `go test ./web/test -v -run TestRESTfulPostRequest` |
| `TestRESTfulGetRequest` | RESTful GET请求 | `go test ./web/test -v -run TestRESTfulGetRequest` |
| `TestPathParameters` | 路径参数绑定 | `go test ./web/test -v -run TestPathParameters` |
| `TestRequiredFieldValidation` | 必填字段验证 | `go test ./web/test -v -run TestRequiredFieldValidation` |
| `TestHeaderBinding` | 请求头绑定 | `go test ./web/test -v -run TestHeaderBinding` |

### 高级功能测试

| 测试名称 | 描述 | 命令 |
|---------|------|------|
| `TestMultipleInterceptors` | 拦截器执行顺序 | `go test ./web/test -v -run TestMultipleInterceptors` |
| `TestRequestTraceID` | 请求追踪ID | `go test ./web/test -v -run TestRequestTraceID` |
| `TestErrorHandling` | 错误处理 | `go test ./web/test -v -run TestErrorHandling` |
| `TestConcurrentRequests` | 并发请求处理 | `go test ./web/test -v -run TestConcurrentRequests` |

### 其他测试

| 测试名称 | 描述 | 命令 |
|---------|------|------|
| `TestHttpServer` | 服务器综合测试 | `go test ./web/test -v -run TestHttpServer` |
| `TestHttpServerInteractive` | 交互式服务器测试 | `go test ./web/test -v -run TestHttpServerInteractive` |

## 常用命令

### 基本测试

```bash
# 运行所有测试，显示详细信息
go test ./web/test -v

# 运行所有测试，显示覆盖率
go test ./web/test -v -cover

# 运行单个测试
go test ./web/test -v -run TestRESTfulPostRequest

# 运行多个相关测试
go test ./web/test -v -run "Test(Restful|Path)"
```

### 性能和分析

```bash
# 基准测试（100次迭代）
go test ./web/test -bench=BenchmarkRESTfulRequest -benchtime=100x

# 基准测试并显示内存分配
go test ./web/test -bench=. -benchmem

# CPU性能分析
go test ./web/test -bench=. -cpuprofile=cpu.prof
go tool pprof cpu.prof

# 内存分析
go test ./web/test -bench=. -memprofile=mem.prof
go tool pprof mem.prof
```

### 覆盖率分析

```bash
# 生成覆盖率报告
go test ./web/test -v -cover -coverprofile=coverage.out

# 查看详细覆盖率
go tool cover -func=coverage.out

# 生成HTML覆盖率报告
go tool cover -html=coverage.out -o coverage.html
```

### 其他有用的命令

```bash
# 运行测试不缓存结果
go test ./web/test -v -count=1

# 设置超时时间
go test ./web/test -v -timeout=30s

# 运行测试并显示日志
go test ./web/test -v -v

# 运行竞态条件检测
go test ./web/test -race

# 并行运行测试
go test ./web/test -v -parallel 4
```

## 测试覆盖范围

### 参数绑定✓
- JSON Body绑定
- 查询参数绑定  
- 路径参数绑定
- 请求头绑定
- 默认值设置

### 参数校验✓
- 必填字段（verf标签）
- 枚举值（inList标签）
- 正则表达式（reg标签）
- 范围验证（between标签）
- 长度验证（len标签）

### 路由和处理✓
- RESTful风格路由
- Action风格路由
- 多路径参数
- 路由匹配

### 拦截器✓
- Before执行
- After执行
- OnPanic处理
- 多个拦截器执行顺序

### 响应和错误✓
- 成功响应格式
- 错误响应格式
- 错误码映射
- 追踪ID管理

### 并发和性能✓
- 并发请求处理
- 连接池管理
- 请求超时
- 性能基准测试

## 故障排除

### 端口已被占用
```bash
# 查找占用端口的进程
lsof -i :9091

# 测试使用的端口：9091, 9092, 9093
# 如果需要修改，编辑integration_test.go中的Port配置
```

### Bean冲突错误
```bash
# 这通常发生在多次运行测试时
# 原因：Bean是全局管理的，需要在测试中清理
# 解决：TestSuite.setup()中已调用bean.ClearBeans()
```

### 超时问题
```bash
# 增加超时时间
go test ./web/test -v -timeout=120s
```

## 最佳实践

1. **孤立测试**：每个测试都在独立的服务器实例中运行
2. **资源清理**：使用defer确保所有资源被清理
3. **Bean管理**：每次setup前清理Bean，避免冲突
4. **等待就绪**：服务器启动后等待500ms确保就绪
5. **并发安全**：HTTP客户端支持并发请求

## 测试结构

```
web/test/
├── integration_test.go      # 集成测试套件（推荐）
├── server_test.go           # 基础服务器测试
├── controller/
│   └── v1/test/test.go      # 测试控制器
├── README.md                # 详细文档
└── TESTING_GUIDE.md         # 本文件
```

## 了解更多

- [Web框架完整使用指南](../../docs/web_usage_guide.md)
- [HTTP客户端文档](../../pkg/http)
- [项目README](../../README.md)

---

**提示**：所有测试命令都应在项目根目录 `/Users/lijinlong/workspace/common-tools` 执行。
