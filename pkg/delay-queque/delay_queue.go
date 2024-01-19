package delayque

import (
	"math"
	"sync"
	"time"

	"github.com/caiflower/common-tools/pkg/sort/heap"
)

type Api interface {
	Offer(delayItem)
	Take() delayItem
	Close()
}

type delayQueue struct {
	l       sync.Mutex
	itemMap map[int64]*delayItem
	heap    heap.Heap[int64]
}

func New() *delayQueue {
	d := &delayQueue{itemMap: make(map[int64]*delayItem), heap: *heap.NewTopMin[int64]()}
	return d
}

func (dq *delayQueue) Offer(item delayItem) {
	dq.l.Lock()
	defer dq.l.Unlock()
	t := item.time.Unix()
	for i := t; i < math.MaxInt64; i++ {
		if _, e := dq.itemMap[i]; !e {
			dq.itemMap[i] = &item
			dq.heap.Add(i)
			break
		}
	}
}

func (dq *delayQueue) Take() (di delayItem) {
	var item *delayItem

	for item == nil {
		if !dq.heap.IsEmpty() {
			dq.l.Lock()
			if !dq.heap.IsEmpty() {
				t, _ := dq.heap.Peek()
				if time.Now().Unix() > t {
					t, err := dq.heap.Pop()
					if err != nil {
						panic(err)
					}
					item = dq.itemMap[t]
					di = *item
					dq.itemMap[t] = nil
				}
			}
			dq.l.Unlock()
		}
		time.Sleep(100 * time.Millisecond)
	}

	return
}

func (dq *delayQueue) Close() {

}
