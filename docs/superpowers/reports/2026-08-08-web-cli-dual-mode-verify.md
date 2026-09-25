# Web CLI Dual Mode 验证报告

- 日期：2026-08-08
- 分支：`feature/20260808/web-cli-dual-mode`
- 验证模式：full（tasks 25、delta spec 2、改动文件 37）
- 结论：PASS

## 摘要

| 维度 | 结果 |
|------|------|
| Completeness | 25/25 tasks，2 个 capability，需求全部有实现与测试覆盖 |
| Correctness | 需求/场景逐项核对通过，核心验收场景有真实测试 |
| Coherence | 设计文档与实现一致，delta spec 已同步远端降级场景 |

## 检查项

| 检查项 | 结果 |
|--------|------|
| tasks.md 全部完成 | PASS（25/25） |
| 实现符合 design.md 高层设计 | PASS（RouteInfo/CLIRoute/资源管理器/serve/routes/缓存） |
| 实现符合 Superpowers Design Doc | PASS（组件结构与数据流一致） |
| capability spec 场景覆盖 | PASS（route-metadata 7 场景、web-cli 16 场景均有实现与测试） |
| proposal.md 目标满足 | PASS（同二进制双模式、动态命令、/cli/routes、缓存、资源管理器接入） |
| delta spec 与设计文档无矛盾 | PASS（新增“远端失败降级本地”场景已同步到设计文档） |
| 设计文档可定位 | PASS（`docs/superpowers/specs/2026-08-08-web-cli-dual-mode-design.md`） |

## 验证命令

- `go build ./...`：PASS
- `go vet ./web/cli/... ./web/router/...`：PASS
- `go test -race ./web/... -count=1`：PASS
- `gofmt -l web/cli/`：无输出

## 本轮修复记录

首次验证发现两处问题，均已修复：

1. 远端命令生成偏差（CRITICAL）：`--server` 之前只切换请求目标，命令树始终来自本地元数据。按用户决策实现“远端优先、本地兜底”：`--server` 时优先拉取/读缓存 `/cli/routes` 元数据生成命令；拉取失败或无 `--server` 时降级本地元数据并输出警告。实现位于 `web/cli/metadata.go`、`web/cli/root.go`，新增真实场景测试 `TestRemoteServerGeneratesCommands`（本地不注册路由）和 `TestRemoteDiscoveryFallsBackToLocal`。
2. 数据竞态（CRITICAL）：`TestClientExecuteBuildsRequest` 中 handler goroutine 与测试 goroutine 共享请求断言变量，`go test -race` 可稳定复现。修复为在 handler goroutine 内断言；`discovery_test.go` 的请求计数改为 `atomic.Int32`。`go test -race ./web/... -count=1` 通过。

## 结论

无 CRITICAL/IMPORTANT 遗留问题，全部检查通过，可进入归档。
