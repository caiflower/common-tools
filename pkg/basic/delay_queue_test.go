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

 package basic

import (
	"fmt"
	"testing"
	"time"
)

type testDelayQueue struct {
	Name string
}

func TestDelayQueue(t *testing.T) {
	queue := NewDelayQueue()
	fmt.Println(time.Now().Format("2006-01-02 15:04:05"))
	queue.Add(&testDelayQueue{Name: "10"}, time.Now().Add(10*time.Second))
	queue.Add(&testDelayQueue{Name: "8"}, time.Now().Add(8*time.Second))
	go func() {
		for {
			value := queue.Take()
			fmt.Printf("time = %s, value = %s\n", time.Now().Format("2006-01-02 15:04:05"), value.(*testDelayQueue).Name)
		}
	}()
	queue.Add(&testDelayQueue{Name: "5"}, time.Now().Add(5*time.Second))
	queue.Add(&testDelayQueue{Name: "4"}, time.Now().Add(4*time.Second))
	queue.Add(&testDelayQueue{Name: "3"}, time.Now().Add(3*time.Second))

	time.Sleep(20 * time.Second)

}
