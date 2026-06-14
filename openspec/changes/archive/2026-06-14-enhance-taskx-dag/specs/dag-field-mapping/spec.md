## ADDED Requirements

### Requirement: 字段映射定义
系统 SHALL 支持在添加数据边时指定字段映射（FieldMapping），定义源节点的输出字段到目标节点输入字段的映射关系。

#### Scenario: 添加带字段映射的数据边
- **WHEN** 用户调用 AddDataEdge(A, B, FieldMapping{SourceField: "result.name", TargetField: "displayName"})
- **THEN** 节点 A 完成后，其输出中 "result.name" 字段的值被映射到节点 B 输入的 "displayName" 字段

#### Scenario: 不指定字段映射时传递完整输出
- **WHEN** 用户调用 AddDataEdge(A, B) 不指定 FieldMapping
- **THEN** 节点 A 的完整输出作为节点 B 的输入

### Requirement: 多前驱数据合并
当节点有多个数据前驱时，系统 SHALL 将所有前驱的输出按字段映射合并为一个 map 作为当前节点的输入。

#### Scenario: 两个前驱的数据合并
- **WHEN** 节点 C 有数据前驱 A（输出 {name: "foo"}）和 B（输出 {age: 20}），无字段映射
- **THEN** 节点 C 的输入为 {A: {name: "foo"}, B: {age: 20}}

#### Scenario: 带字段映射的多前驱合并
- **WHEN** 节点 C 有数据前驱 A 和 B，A 映射 {name → displayName}，B 映射 {age → userAge}
- **THEN** 节点 C 的输入为 {displayName: "foo", userAge: 20}

### Requirement: 静态值注入
系统 SHALL 支持为节点设置静态值，这些值在编译时确定，运行时合并到节点输入中。

#### Scenario: 设置静态值
- **WHEN** 用户调用 node.SetStaticValue(FieldPath{"config"}, "default-config")
- **THEN** 节点执行时，其输入中包含 config: "default-config" 字段
