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
