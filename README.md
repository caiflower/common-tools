# common-tools

| 名称            | 路径                                              | 描述                       | 修订                                | 修订时间      |
|---------------|-------------------------------------------------|--------------------------| ----------------------------------- |-----------|
| [堆排序](#堆排序)   | github.com/caiflower/common-tools/pkg/sort/heap | 简单的堆结构，使用了go泛型，支持常用的基本类型 | 修复了交替执行Pop和Add方法产生的BUG | 2024-1-19 |
| [延迟队列](#延迟队列) | github.com/caiflower/common-tools/pkg/queue     | 延迟队列                     | -                                   | 2024-1-19 |
| [自旋锁](#自旋锁)   | github.com/caiflower/common-tools/syncx         | 自旋锁，拷贝自ants项目            |                                     | 2024-2-5  |
| 依赖注入          | github.com/caiflower/common-tools/bean          | 自动注入ptr                  |                                     | 2024-9-21 |

# 堆排****序

```go
func testTopMin(total int) {
    nums := make([]int, 0)

    heap := NewTopMin[int]()
    for j := total; j >= 0; j-- {
       // 随机生成数字
       rn := rand.Intn(total)
       heap.Add(rn)
       nums = append(nums, rn)
    }

    // 使用官方排序
    sort.Ints(nums)
    for j := 0; j <= total; j++ {
       pop, err := heap.Pop()
       if err != nil {
          panic(err)
       }
      
       // 对比结果
       if nums[j] != pop {
          panic(fmt.Sprintf("result error, correct: %v, error: %v", nums[j], pop))
       } else {
          fmt.Printf("pop: %v\n", pop)
       }
    }
}
```

# 延迟队列

```go
package main

import (
    "fmt"
    "strconv"
    "sync"
    "time"

    delayque "github.com/caiflower/common-tools/pkg/queue"
)

type MyData struct {
    Name string
}

func main() {
    wait := sync.WaitGroup{}
    queue := delayque.NewDelayQueue()
    total := 10
    consumerCnt := 3

    wait.Add(total)
    for i := 0; i < consumerCnt; i++ {
       go func() {
          for {
             d := queue.Take()
             data := d.GetData().(MyData)
             fmt.Printf("now: %v, dataName: %v\n", time.Now().Format(time.DateTime), data.Name)
             wait.Done()
          }

       }()
    }

    fmt.Printf("offer time: %v\n", time.Now().Format(time.DateTime))
    for i := 0; i < total; i++ {
       queue.Offer(delayque.NewDelayItem(MyData{Name: strconv.Itoa(i)}, time.Now().Add(time.Second*3)))
    }

    wait.Wait()
}
```

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