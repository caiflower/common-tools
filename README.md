# common-tools

| 名称            | 路径                                              | 描述                       | 修订                                | 修订时间      |
|---------------|-------------------------------------------------|--------------------------| ----------------------------------- |-----------|
| [自旋锁](#自旋锁)   | github.com/caiflower/common-tools/syncx         | 自旋锁，拷贝自ants项目            |                                     | 2024-2-5  |
| 依赖注入          | github.com/caiflower/common-tools/bean          | 自动注入ptr                  |                                     | 2024-9-21 |


# 自旋锁

```go
package main

import (
    "fmt"
    "github.com/caiflower/common-tools/pkg/syncx"
    "sync"
)

func main() {
    wait := sync.WaitGroup{}
    lock := syncx.NewSpinLock()
    var num int

    fn := func() {
        lock.Lock()
        num++
        lock.Unlock()
        wait.Done()
    }

    for i := 0; i < 10000; i++ {
        wait.Add(1)
        go fn()
    }

    wait.Wait()
    fmt.Printf("-----num=%v-----", num)
}
```