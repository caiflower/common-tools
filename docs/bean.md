## 概要

 设计初衷时为了更好的管理服务中的单例对象，简化初始化服务的复杂度，使用方法类似于Java的依赖注入，对于有过Java经验的程序员非常容易入手。

| 工具名 | 使用方式                                                     |                                                             |
| ------ | ------------------------------------------------------------ | ----------------------------------------------------------- |
| wire   | 首先安装wire工具，其次要维护gen文件，生成的自动注入文件在大项目中会非常大，虽然能看看传参的过程，但是可读性差 | github.com/google/wire                                      |
| bean   | 将实例交给bean托管，通过注解注入，使用方式简单               | [*github.com/caiflower/common-tools/pkg/bean*](../pkg/bean) |

## 示例

```Go
package beantest

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

type TestAutoWrite struct {
    *TestAutoWrite1 `autowired:""`
    TestAutoWrite2  *TestAutoWrite2 `autowired:""`
}

type TestAutoWrite1 struct {
}

type TestAutoWrite2 struct {
    TestAutoWrite3 *TestAutoWrite3 `autowired:""`
}

type TestAutoWrite3 struct {
    TestAutoWrite2 *TestAutoWrite2 `autowired:""`
    TestAutoWrite4 TestAutoWrite4  `autowired:""`
}

type TestAutoWrite4 interface {
    TestNameXxx() string
}

type testAutoWrite4 struct {
}

func (t *testAutoWrite4) TestNameXxx() string {
    return "testAutoWrite4"
}

func TestIoc(t *testing.T) {
    // 依赖注入
    test := &TestAutoWrite{}
    test1 := &TestAutoWrite1{}
    test2 := &TestAutoWrite2{}
    test3 := &TestAutoWrite3{}
    test4 := &testAutoWrite4{}
    AddBean(test)
    AddBean(test1)
    AddBean(test2)
    AddBean(test3)
    AddBean(test4)

    // Ioc
    Ioc()

    assert.Same(t, test.TestAutoWrite1, test1)
    assert.Same(t, test.TestAutoWrite2, test2)
    assert.Same(t, test2.TestAutoWrite3, test3)
    assert.Same(t, test3.TestAutoWrite2, test2)
    assert.Same(t, test3.TestAutoWrite4, test4)
}
```