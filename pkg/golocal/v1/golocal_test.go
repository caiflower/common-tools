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

package v1

import (
	"strconv"
	"testing"
)

func TestTraceID(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		go func(i int) {
			PutTraceID(strconv.Itoa(i))
			traceID := GetTraceID()
			if traceID != strconv.Itoa(i) {
				panic("traceID is error")
			}
		}(i)
	}
}

func TestPut(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		go func(i int) {
			Put("test", i)
			get := Get("test")
			if get.(int) != i {
				panic("put is error")
			}
		}(i)
	}
}

// OLD: BenchmarkFib-11    	 1000000	      1210 ns/op
// CURRENT: BenchmarkFib-11    	 2455628	       616.7 ns/op
func BenchmarkFib(b *testing.B) {
	for n := 0; n < b.N; n++ {
		go func(i int) {
			PutTraceID(strconv.Itoa(i))
			traceID := GetTraceID()
			if traceID != strconv.Itoa(i) {
				panic("traceID is error")
			}
		}(n)
	}
}
