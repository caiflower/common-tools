## ADDED Requirements

### Requirement: 回滚策略类型
系统 SHALL 支持三种回滚策略：RollbackAll（回滚所有已执行子任务）、RollbackFailed（只回滚失败子任务）、RollbackCustom（自定义回滚逻辑）。

#### Scenario: 默认使用 RollbackAll 策略
- **WHEN** 用户未设置回滚策略
- **THEN** 子任务失败时，所有已成功执行的子任务按逆拓扑序回滚

#### Scenario: 设置 RollbackFailed 策略
- **WHEN** 用户设置 task.SetRollbackStrategy(RollbackFailed)，且子任务 A 执行失败
- **THEN** 仅对子任务 A 执行回滚，其他已成功子任务不回滚

#### Scenario: 设置 RollbackCustom 策略
- **WHEN** 用户设置 task.SetRollbackStrategy(RollbackCustom) 并提供自定义回滚函数
- **THEN** 子任务失败时，调用自定义回滚函数确定需要回滚的子任务列表

### Requirement: 自定义回滚函数
RollbackCustom 策略 SHALL 接受一个自定义函数，该函数接收已完成节点列表和失败节点，返回需要回滚的节点列表。

#### Scenario: 自定义回滚函数返回回滚列表
- **WHEN** 自定义回滚函数返回 [B, C]（B 和 C 需要回滚）
- **THEN** 系统对 B 和 C 执行回滚操作

#### Scenario: 自定义回滚函数返回空列表
- **WHEN** 自定义回滚函数返回空列表
- **THEN** 不执行任何回滚操作

### Requirement: 回滚执行顺序
回滚 SHALL 按逆拓扑序执行，即后执行的子任务先回滚。

#### Scenario: 逆拓扑序回滚
- **WHEN** 子任务执行顺序为 A → B → C，C 失败需要回滚 A 和 B
- **THEN** 回滚顺序为 B → A
