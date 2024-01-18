package heap

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

func BenchmarkRun(b *testing.B) {
	for i := 0; i < b.N; i++ {
		total := 1000 * (i + 1)

		testTopMin(total)
		testTopMax(total)
	}
}

func testTopMin(total int) {
	nums := make([]int, 0)

	heap := NewTopMin[int]()
	for j := total; j >= 0; j-- {
		rn := rand.Intn(total)
		heap.Add(rn)
		nums = append(nums, rn)
	}

	sort.Ints(nums)
	for j := 0; j <= total; j++ {
		pop, err := heap.Pop()
		if err != nil {
			panic(err)
		}
		if nums[j] != pop {
			panic(fmt.Sprintf("result error, correct: %v, error: %v", nums[j], pop))
		} else {
			fmt.Printf("pop: %v\n", pop)
		}
	}
}

func TestNewTopMin(t *testing.T) {
	for i := 0; i < 10; i++ {
		total := 1000 * (i + 1)
		nums := make([]int, 0)

		heap := NewTopMin[int]()
		for j := total; j >= 0; j-- {
			rn := rand.Intn(total)
			//rn := i
			heap.Add(rn)
			nums = append(nums, rn)
		}

		sort.Ints(nums)
		for j := 0; j <= total; j++ {
			pop, err := heap.Pop()
			if err != nil {
				panic(err)
			}
			if nums[j] != pop {
				panic("result error")
			} else {
				fmt.Printf("pop: %v\n", pop)
			}
		}
	}
}

func testTopMax(total int) {
	heap := NewTopMax[int]()
	nums := make([]int, 0)

	for i := total; i >= 0; i-- {
		rn := rand.Intn(total)
		heap.Add(rn)
		nums = append(nums, rn)
	}

	sort.Ints(nums)
	for j := total; j >= 0; j-- {
		pop, err := heap.Pop()
		if err != nil {
			panic(err)
		}
		if nums[j] != pop {
			panic(fmt.Sprintf("result error, correct: %v, error: %v", nums[j], pop))
		} else {
			fmt.Printf("pop: %v\n", pop)
		}
	}
}

func TestNewTopMax(t *testing.T) {
	testTopMax(10000)
}
