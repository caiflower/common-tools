package basic

import (
	"fmt"
	"strconv"
	"testing"
)

type objectHeapItem struct {
	priority int
	Name     string
}

func (o objectHeapItem) String() string {
	return strconv.Itoa(o.priority)
}

func TestObjectHeap(t *testing.T) {

	h := ObjectPriorityQueue[objectHeapItem]{
		Max: false,
	}
	h.Offer(objectHeapItem{priority: 1, Name: "1"})
	h.Offer(objectHeapItem{priority: 2, Name: "2"})
	h.Offer(objectHeapItem{priority: 3, Name: "3"})
	h.Offer(objectHeapItem{priority: 4, Name: "4"})

	//
	for i := 0; i < 4; i++ {
		poll, err := h.Poll()
		if err != nil {
			panic(err)
		}
		fmt.Println(poll.Name)
	}
}
