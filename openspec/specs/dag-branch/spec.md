## ADDED Requirements

### Requirement: 条件分支定义
系统 SHALL 支持在 DAG 节点上挂载条件分支（Branch），分支包含一个条件函数和一组合法的目标节点。

#### Scenario: 添加条件分支
- **WHEN** 用户调用 AddBranch(nodeKey, branch)，branch 包含条件函数和 endNodes
- **THEN** 系统在指定节点上注册分支，分支的目标节点被添加到图中

#### Scenario: 分支条件返回有效目标节点
- **WHEN** 分支节点执行完成后，条件函数返回 endNodes 中的一个节点 key
- **THEN** 系统仅调度该目标节点执行，其余 endNodes 标记为 Skipped

#### Scenario: 分支条件返回无效目标节点
- **WHEN** 分支节点执行完成后，条件函数返回的节点 key 不在 endNodes 中
- **THEN** 系统 SHALL 返回错误

### Requirement: 分支节点的 Skip 传播
当分支选择某条路径后，未被选中的路径上的节点 SHALL 自动标记为 Skipped，Skip 状态 SHALL 沿控制依赖向下传播。

#### Scenario: 未选中分支节点被跳过
- **WHEN** 分支选择了路径 A，路径 B 的起始节点未被选中
- **THEN** 路径 B 的起始节点标记为 Skipped

#### Scenario: Skip 状态沿控制依赖传播
- **WHEN** 节点 C 的唯一控制前驱是路径 B 上的节点 D，且 D 被标记为 Skipped
- **THEN** 节点 C 也标记为 Skipped

#### Scenario: Skip 不影响汇聚节点
- **WHEN** 汇聚节点 E 有两个控制前驱（路径 A 的节点 F 和路径 B 的节点 G），G 被跳过但 F 已完成
- **THEN** 节点 E 不被跳过，正常执行

### Requirement: 分支数据流
分支节点 SHALL 将其输出数据传递给被选中的目标节点，未选中的目标节点不接收数据。

#### Scenario: 选中节点接收分支数据
- **WHEN** 分支选择了目标节点 B
- **THEN** 分支节点的输出数据传递给节点 B

#### Scenario: 未选中节点不接收数据
- **WHEN** 分支未选择目标节点 C
- **THEN** 节点 C 不接收分支节点的数据，且标记为 Skipped
