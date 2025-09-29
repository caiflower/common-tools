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
	"testing"
)

func Test_DoFuncString(t *testing.T) {
	slices := []string{"1", "2", "3", "4", "5"}
	fn := func(v interface{}) {
		fmt.Println(v)
	}
	err := DoFuncString(10, fn, slices...)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v", slices)
}

type Object struct {
	Age int
}

func Test_DoFuncInterface(t *testing.T) {
	var slices []*Object
	slices = append(slices, &Object{}, &Object{})
	fn := func(v interface{}) {
		object := v.(*Object)
		object.Age = 1
	}

	m := make(map[int]interface{})
	for i, v := range slices {
		m[i] = v
	}
	err := DoFuncInterface(10, fn, m)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v %v", slices[0], slices[1])
}

func Test_DoFunc(t *testing.T) {
	var slices []*Object
	slices = append(slices, &Object{}, &Object{}, &Object{})
	fn := func(v *Object) {
		v.Age = 1
	}

	err := DoFunc[*Object](10, fn, slices...)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v %v %v", slices[0], slices[1], slices[2])
}
