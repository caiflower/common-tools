package pool

import (
	"fmt"
	"sync"
)

func DoFuncString(poolSize int, fn func(interface{}), slices ...string) error {
	if fn == nil {
		return fmt.Errorf("nil func error")
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(len(slices))
	c := make(chan string, 100)
	defer close(c)
	go func() {
		for _, v := range slices {
			c <- v
		}
	}()

	if poolSize <= 0 || poolSize > len(slices) {
		poolSize = len(slices)
	}
	for i := 0; i < poolSize; i++ {
		go func() {
			for v := range c {
				fn(v)
				waitGroup.Done()
			}
		}()
	}

	waitGroup.Wait()
	return nil
}

func DoFuncInterface(poolSize int, fn func(interface{}), slices map[int]interface{}) error {
	if fn == nil {
		return fmt.Errorf("nil func error")
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(len(slices))
	c := make(chan interface{}, 100)
	defer close(c)
	go func() {
		for _, v := range slices {
			c <- v
		}
	}()

	if poolSize <= 0 || poolSize > len(slices) {
		poolSize = len(slices)
	}
	for i := 0; i < poolSize; i++ {
		go func() {
			for v := range c {
				fn(v)
				waitGroup.Done()
			}
		}()
	}

	waitGroup.Wait()
	return nil
}
