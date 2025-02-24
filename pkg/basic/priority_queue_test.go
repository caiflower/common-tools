package basic

import (
	"fmt"
	"testing"
)

func TestHeap_Contains(t *testing.T) {
	nums := []int{4, 5, 1, 6, 2, 7, 3, 8, 10, 20, 13, 30, 10}
	h := PriorityQueue[int]{
		Max: false,
	}

	for i := 0; i < len(nums); i++ {
		h.Offer(nums[i])
	}

	for i := 0; i < len(nums); i++ {
		poll, err := h.Poll()
		if err != nil {
			panic(err)
		}
		fmt.Println(poll)
	}
}
