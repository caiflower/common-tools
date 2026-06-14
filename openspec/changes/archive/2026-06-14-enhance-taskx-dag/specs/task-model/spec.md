## MODIFIED Requirements

### Requirement: Task 图结构管理
Task SHALL 使用自建的 dagGraph 替代 dominikbraun/graph，支持控制边和数据边的独立管理。子任务完成后 SHALL 更新节点状态而非删除节点和边。执行器注册 SHALL 为实例级而非全局变量。

#### Scenario: 创建 Task
- **WHEN** 用户调用 NewTask("my-task")
- **THEN** Task 包含空的 dagGraph 和实例级的 executorManager

#### Scenario: 添加子任务到 Task
- **WHEN** 用户调用 task.AddSubtask(subtask)
- **THEN** 子任务被添加到 dagGraph 中，节点状态初始化为 Pending

#### Scenario: 添加控制边
- **WHEN** 用户调用 task.AddControlEdge(src, dst)
- **THEN** dagGraph 中添加一条从 src 到 dst 的控制边

#### Scenario: 添加数据边
- **WHEN** 用户调用 task.AddDataEdge(src, dst, mappings...)
- **THEN** dagGraph 中添加一条从 src 到 dst 的数据边

#### Scenario: 添加同时包含控制和数据的边
- **WHEN** 用户调用 task.AddEdge(src, dst)
- **THEN** dagGraph 中同时添加控制边和数据边

#### Scenario: 子任务成功完成后更新状态
- **WHEN** 子任务执行成功
- **THEN** 节点状态更新为 Succeeded，dagChannel 的控制依赖和数据依赖标记为 Ready，图结构不变

#### Scenario: 子任务失败后更新状态
- **WHEN** 子任务执行失败
- **THEN** 节点状态更新为 Failed，根据回滚策略执行回滚

#### Scenario: 实例级执行器注册
- **WHEN** 用户调用 task.RegisterTaskExecutor(executor, subtaskExecutors)
- **THEN** 执行器注册到该 Task 实例的 executorManager，不影响其他 Task 实例

### Requirement: 节点触发模式
每个子任务 SHALL 支持设置触发模式：AllPredecessor（所有前驱完成才触发，默认）或 AnyPredecessor（任一前驱完成即触发）。

#### Scenario: AllPredecessor 模式（默认）
- **WHEN** 子任务的触发模式为 AllPredecessor，且存在未完成的前驱节点
- **THEN** 子任务不可执行

#### Scenario: AllPredecessor 模式所有前驱完成
- **WHEN** 子任务的触发模式为 AllPredecessor，且所有前驱节点均已完成或跳过
- **THEN** 子任务可被调度执行

#### Scenario: AnyPredecessor 模式任一前驱完成
- **WHEN** 子任务的触发模式为 AnyPredecessor，且至少一个前驱节点已完成
- **THEN** 子任务可被调度执行

### Requirement: 节点 Skip 状态
子任务 SHALL 支持 Skipped 状态。当节点被跳过时，其输出为空，后继节点根据触发模式决定是否执行。

#### Scenario: 手动跳过节点
- **WHEN** 调用 skipSubtask(nodeKey)
- **THEN** 节点状态更新为 Skipped，dagChannel 报告 Skip 给后继节点

#### Scenario: Skip 传播到无其他前驱的后继
- **WHEN** 节点 A 被跳过，且后继节点 B 的所有控制前驱均被跳过
- **THEN** 节点 B 也自动标记为 Skipped

### Requirement: NextSubTasks 查询可执行节点
系统 SHALL 通过查询 dagChannel 状态获取当前可执行的节点列表，按优先级降序排列。

#### Scenario: 查询可执行节点
- **WHEN** 调用 task.NextSubTasks()
- **THEN** 返回所有 dagChannel 报告依赖就绪的 Pending 状态节点，按优先级降序排列

#### Scenario: 无可执行节点
- **WHEN** 所有 Pending 节点的 dagChannel 均报告依赖未就绪
- **THEN** 返回空列表

#### Scenario: 所有节点已完成或跳过
- **WHEN** 所有节点状态为 Succeeded、Failed 或 Skipped
- **THEN** 任务判定为完成

### Requirement: 优先级调度
子任务 SHALL 支持设置优先级，调度时优先执行高优先级节点。

#### Scenario: 设置子任务优先级
- **WHEN** 用户调用 subtask.SetPriority(10)
- **THEN** 该子任务的优先级为 10，NextSubTasks 返回时排在优先级较低的节点前面

#### Scenario: 默认优先级为 0
- **WHEN** 用户未设置子任务优先级
- **THEN** 子任务优先级为 0

### Requirement: 图可视化
系统 SHALL 支持输出 DOT 和 Mermaid 格式的图描述。

#### Scenario: 输出 DOT 格式
- **WHEN** 用户调用 task.GraphDOT()
- **THEN** 返回 Graphviz DOT 格式的图描述字符串

#### Scenario: 输出 Mermaid 格式
- **WHEN** 用户调用 task.GraphMermaid()
- **THEN** 返回 Mermaid 格式的图描述字符串

#### Scenario: 可视化包含边类型信息
- **WHEN** 图中存在控制边和数据边
- **THEN** DOT/Mermaid 输出中标注边的类型（control/data/control+data）

### Requirement: 泛型类型安全
Subtask SHALL 支持泛型输入输出类型标注，Compile 阶段校验节点间类型兼容。

#### Scenario: 创建泛型子任务
- **WHEN** 用户调用 NewSubtask[MyInput, MyOutput]("step1")
- **THEN** 子任务的 inputType 为 MyInput，outputType 为 MyOutput

#### Scenario: Compile 时类型不兼容报错
- **WHEN** 节点 A 输出类型为 string，节点 B 输入类型为 int，且无字段映射
- **THEN** Compile() 返回类型不兼容错误

#### Scenario: Compile 时类型兼容通过
- **WHEN** 节点 A 输出类型为 MyOutput，节点 B 输入类型为 MyOutput
- **THEN** Compile() 类型校验通过
