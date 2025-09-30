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
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestAutoWrite struct {
	*TestAutoWrite1 `autowired:""`
	TestAutoWrite2  *TestAutoWrite2 `autowired:""`
}

type TestAutoWrite1 struct {
}

type TestAutoWrite2 struct {
	TestAutoWrite3 *TestAutoWrite3 `autowired:""`
}

type TestAutoWrite3 struct {
	TestAutoWrite2 *TestAutoWrite2 `autowired:""`
	TestAutoWrite4 TestAutoWrite4  `autowired:""`
}

type TestAutoWrite4 interface {
	TestNameXxx() string
}

type testAutoWrite4 struct {
}

func (t *testAutoWrite4) TestNameXxx() string {
	return "testAutoWrite4"
}

func TestIoc(t *testing.T) {
	test := &TestAutoWrite{}
	test1 := &TestAutoWrite1{}
	test2 := &TestAutoWrite2{}
	test3 := &TestAutoWrite3{}
	test4 := &testAutoWrite4{}
	AddBean(test)
	AddBean(test1)
	AddBean(test2)
	AddBean(test3)
	AddBean(test4)

	Ioc()

	assert.Same(t, test.TestAutoWrite1, test1)
	assert.Same(t, test.TestAutoWrite2, test2)
	assert.Same(t, test2.TestAutoWrite3, test3)
	assert.Same(t, test3.TestAutoWrite2, test2)
	assert.Same(t, test3.TestAutoWrite4, test4)
}
