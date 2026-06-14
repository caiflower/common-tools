## ADDED Requirements

### Requirement: 子图节点嵌入
系统 SHALL 支持将一个 Task 作为子图节点嵌入另一个 Task。子图在外层 Task 中表现为一个普通节点。

#### Scenario: 添加子图节点
- **WHEN** 用户调用 mainTask.AddSubtaskGraph("sub-node", subTask)
- **THEN** subTask 作为一个整体节点嵌入 mainTask，key 为 "sub-node"

#### Scenario: 子图节点接收外层数据
- **WHEN** 外层节点 A 通过 AddDataEdge(A, subGraphNode) 连接到子图节点
- **THEN** 节点 A 的输出作为子图的输入

#### Scenario: 子图节点输出传递给外层后继
- **WHEN** 外层通过 AddDataEdge(subGraphNode, B) 连接子图节点到节点 B
- **THEN** 子图的最终输出作为子图节点的输出传递给节点 B

### Requirement: 子图执行
子图节点执行时 SHALL 先编译子图，然后按子图内部 DAG 逻辑执行，子图整体完成后将最终输出作为节点输出。

#### Scenario: 子图内部按 DAG 逻辑执行
- **WHEN** 子图节点被调度执行
- **THEN** 系统编译子图，按子图内部依赖关系依次执行子任务

#### Scenario: 子图内部失败导致子图节点失败
- **WHEN** 子图内部某个子任务执行失败
- **THEN** 子图节点状态标记为 Failed，外层 DAG 根据失败状态处理

#### Scenario: 子图全部成功则子图节点成功
- **WHEN** 子图内部所有子任务执行成功
- **THEN** 子图节点状态标记为 Succeeded，子图的最终输出作为节点输出

### Requirement: 子图编译
子图 SHALL 在外层 Task Compile 时一并编译，编译时校验子图内部合法性。

#### Scenario: 外层编译时编译子图
- **WHEN** 外层 Task 调用 Compile()
- **THEN** 子图也被编译，子图的环检测和类型校验一并执行

#### Scenario: 子图编译失败导致外层编译失败
- **WHEN** 子图内部存在环或类型不兼容
- **THEN** 外层 Compile() 返回子图编译错误
