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
)

func TestSet(t *testing.T) {
	s := Set[int]{}

	s.Add(1)
	s.Add(2)
	s.Add(3)
	s.Add(4)

	fmt.Println(s.Size() == 4)
	fmt.Println(s.Contains(1) == true)
	fmt.Println(s.Contains(4) == true)
	fmt.Println(s.Contains(5) == false)

	fmt.Println(s.Remove(5) == false)
	fmt.Println(s.Remove(4) == true)

	fmt.Println(s.Size() == 3)
	fmt.Println(s.Contains(4) == false)
	fmt.Println(s.Contains(1) == true)
	fmt.Println(s.Contains(2) == true)
	fmt.Println(s.Contains(3) == true)

	nums := []int{1, 2, 3}
	for _, i := range s.ToSlice() {
		find := false
		for _, j := range nums {
			if i == j {
				find = true
			}
		}
		fmt.Println(find)
	}

	s.Clear()
	fmt.Println(s.Size() == 0)

	fmt.Println(len(s.ToSlice()) == 0)
}
