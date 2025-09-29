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

 package bean

import (
	"fmt"
	"testing"
)

type TestAutoWrite struct {
	*TestAutoWrite1 `autowired:""`
	TestAutoWrite2  *TestAutoWrite2 `autowired:""`
	//TestAutoWrite4 TestAutoWrite4 `autowired:""`
}

type TestAutoWrite1 struct {
	Name int
}

type TestAutoWrite2 struct {
	TestAutoWrite3 *TestAutoWrite3 `autowired:""`
}

type TestAutoWrite3 struct {
	TestAutoWrite2 *TestAutoWrite2 `autowired:""`
}

type TestAutoWrite4 struct {
}

var test = &TestAutoWrite{}

func addBean() {
	AddBean(test)
	AddBean(&TestAutoWrite1{})
	AddBean(&TestAutoWrite2{})
	AddBean(&TestAutoWrite3{})
	AddBean(&TestAutoWrite4{})
}

func TestIoc(t *testing.T) {
	addBean()
	Ioc()

	fmt.Print(test)
}
