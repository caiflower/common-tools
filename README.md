# common-tools

| 名称              | 路径                                            | 功能                                           | 修订时间  |
| ----------------- | ----------------------------------------------- | ---------------------------------------------- | --------- |
| [堆排序](#堆排序) | github.com/caiflower/common-tools/pkg/sort/heap | 简单的堆结构，使用了go泛型，支持常用的基本类型 | 2024-1-18 |

# 堆排序

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