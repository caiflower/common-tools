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
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testDelayQueue struct {
	Name string
}

func TestDelayQueue(t *testing.T) {
	queue := NewDelayQueue()
	begin := time.Now()
	cnt := 0
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10000; i++ {
		intn := r.Intn(10)
		go queue.Add(&testDelayQueue{Name: fmt.Sprintf("%v", intn)}, begin.Add(time.Duration(intn)*time.Second))
	}

	for {
		value := queue.Take()
		atoi, _ := strconv.Atoi(value.(*testDelayQueue).Name)
		assert.Equal(t, true, time.Now().After(begin.Add(time.Second*time.Duration(atoi))))
		cnt++
		if queue.Size() == 0 {
			break
		}
	}

	assert.Equal(t, 10000, cnt)
}
