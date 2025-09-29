/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

func DoFunc[T any](poolSize int, fn func(T), slices ...T) error {
	if fn == nil {
		return fmt.Errorf("nil func error")
	}

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(len(slices))
	c := make(chan T, 100)
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
