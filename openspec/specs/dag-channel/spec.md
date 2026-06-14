## ADDED Requirements

### Requirement: DAG 通道管理控制依赖状态
系统 SHALL 为每个 DAG 节点维护一个 dagChannel，跟踪其控制前驱节点的依赖状态（Waiting/Ready/Skipped）。节点仅在所有控制前驱满足触发条件时才可执行。

#### Scenario: 所有控制前驱完成时节点就绪
- **WHEN** 节点 A 的所有控制前驱节点状态变为 Ready
- **THEN** 节点 A 的 dagChannel 报告依赖就绪，节点 A 可被调度执行

#### Scenario: 控制前驱仍在等待时节点阻塞
- **WHEN** 节点 A 存在至少一个控制前驱节点状态为 Waiting
- **THEN** 节点 A 的 dagChannel 报告依赖未就绪，节点 A 不可被调度

#### Scenario: 控制前驱跳过时传播 Skip
- **WHEN** 节点 A 的所有控制前驱节点状态均为 Skipped
- **THEN** 节点 A 自动标记为 Skipped，不执行

#### Scenario: 控制前驱部分跳过时节点仍可执行
- **WHEN** 节点 A 的触发模式为 AllPredecessor，且部分控制前驱 Skipped、其余 Ready
- **THEN** 节点 A 的 dagChannel 报告依赖就绪，节点 A 可被调度执行

### Requirement: DAG 通道管理数据依赖和值传递
系统 SHALL 为每个 DAG 节点维护数据前驱的就绪状态和传递的值。数据前驱就绪后，其输出值 SHALL 存储在 dagChannel 的 values 映射中。

#### Scenario: 数据前驱完成后值被记录
- **WHEN** 数据前驱节点 B 完成执行并产生输出
- **THEN** 节点 A 的 dagChannel 将 B 标记为数据就绪，并存储 B 的输出值

#### Scenario: 所有数据前驱就绪后可获取合并输入
- **WHEN** 节点 A 的所有数据前驱均标记为就绪
- **THEN** dagChannel.get() 返回合并后的输入数据和 true 标志

#### Scenario: 数据前驱未就绪时无法获取输入
- **WHEN** 节点 A 存在至少一个数据前驱未就绪
- **THEN** dagChannel.get() 返回 nil 和 false 标志

### Requirement: 控制依赖与数据依赖独立管理
系统 SHALL 支持控制边（ControlEdge）和数据边（DataEdge）的独立添加和管理。控制边决定执行顺序，数据边决定数据流向。

#### Scenario: 仅添加控制边
- **WHEN** 用户调用 AddControlEdge(A, B)
- **THEN** 节点 B 等待节点 A 完成后才执行，但 A 的输出不传递给 B

#### Scenario: 仅添加数据边
- **WHEN** 用户调用 AddDataEdge(A, B)
- **THEN** 节点 A 的输出传递给节点 B，但 B 不因 A 的完成状态而阻塞

#### Scenario: 同时添加控制和数据边
- **WHEN** 用户调用 AddEdge(A, B)
- **THEN** 等效于同时调用 AddControlEdge(A, B) 和 AddDataEdge(A, B)，B 等待 A 完成并接收 A 的输出

### Requirement: 非破坏性状态管理
系统 SHALL 在子任务完成后保留图结构，通过更新节点状态而非删除节点和边来跟踪执行进度。

#### Scenario: 子任务成功完成后图结构不变
- **WHEN** 子任务 A 执行成功
- **THEN** 节点 A 的状态更新为 Succeeded，图中的节点和边保持完整

#### Scenario: 子任务失败后图结构不变
- **WHEN** 子任务 A 执行失败
- **THEN** 节点 A 的状态更新为 Failed，图中的节点和边保持完整

#### Scenario: 可查询已完成节点的历史状态
- **WHEN** 子任务 A 已完成（Succeeded 或 Failed）
- **THEN** 系统仍可通过图结构查询 A 的前驱和后继关系
