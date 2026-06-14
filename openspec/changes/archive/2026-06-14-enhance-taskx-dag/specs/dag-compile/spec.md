## ADDED Requirements

### Requirement: 图编译校验
系统 SHALL 提供 Compile() 方法，在图执行前进行完整校验。编译成功后图结构 SHALL 不可变。

#### Scenario: 编译时检测环
- **WHEN** 图中存在环路
- **THEN** Compile() 返回错误，指出环中涉及的节点

#### Scenario: 编译时检测缺少起始节点
- **WHEN** 图中没有入度为 0 的起始节点
- **THEN** Compile() 返回错误

#### Scenario: 编译时检测缺少终止节点
- **WHEN** 图中没有出度为 0 的终止节点
- **THEN** Compile() 返回错误

#### Scenario: 编译成功后图不可变
- **WHEN** Compile() 成功返回 compiledDAG
- **THEN** 后续调用 AddNode/AddEdge/AddBranch SHALL 返回 ErrGraphCompiled 错误

#### Scenario: 编译时校验分支目标节点存在
- **WHEN** 分支的 endNodes 中包含图中不存在的节点 key
- **THEN** Compile() 返回错误

### Requirement: 编译时类型兼容性校验
系统 SHALL 在编译时校验节点间输入输出类型的兼容性。

#### Scenario: 无字段映射时类型必须兼容
- **WHEN** 数据边从节点 A（输出类型 O）到节点 B（输入类型 I），无字段映射
- **THEN** Compile() 校验 O 必须可赋值给 I，否则返回错误

#### Scenario: 有字段映射时源类型必须为 map
- **WHEN** 数据边有字段映射，但前驱节点输出类型不是 map/struct
- **THEN** Compile() 返回错误

#### Scenario: 字段映射目标字段冲突校验
- **WHEN** 多个前驱节点映射到同一目标字段
- **THEN** Compile() 返回错误

### Requirement: 编译产物
编译成功后 SHALL 生成 compiledDAG 对象，包含不可变的图结构、节点通道、分支信息和起止节点列表。

#### Scenario: 编译产物包含完整信息
- **WHEN** Compile() 成功
- **THEN** compiledDAG 包含 dagGraph、channels、branches、startNodes、endNodes

#### Scenario: 编译产物可直接用于执行
- **WHEN** compiledDAG 已生成
- **THEN** 调度器可直接使用 compiledDAG 进行任务调度，无需重新构建图
