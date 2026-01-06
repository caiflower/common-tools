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

package tools

import (
	"testing"

	"github.com/caiflower/common-tools/pkg/tools/bytesconv"
	"github.com/stretchr/testify/assert"
)

func BenchmarkToCamel(b *testing.B) {
	var cases = []string{
		"TestCase",
		"Test",
		"test",
		"Apple",
		"Zoom",
		"testCase",
		"test",
		"test",
		"apple",
		"zoom",
	}

	for i := 0; i < b.N; i++ {
		for _, s := range cases {
			_ = func() bool {
				return len(ToCamel(s)) != 0
			}()
		}
	}
}

func BenchmarkToCamelByte(b *testing.B) {
	var cases = [][]byte{
		[]byte("TestCase"),
		[]byte("Test"),
		[]byte("test"),
		[]byte("Apple"),
		[]byte("Zoom"),
		[]byte("testCase"),
		[]byte("test"),
		[]byte("test"),
		[]byte("apple"),
		[]byte("zoom"),
		[]byte("test_case"),
		[]byte("111111111111111"),
	}

	for i := 0; i < b.N; i++ {
		for _, s := range cases {
			_ = func() bool {
				return len(ToCamelByte(s)) != 0
			}()
		}
	}
}

func TestToCamel(t *testing.T) {
	var cases = [][]byte{
		[]byte("TestCase"),
		[]byte("Test"),
		[]byte("test"),
		[]byte("Apple"),
		[]byte("Zoom"),
		[]byte("testCase"),
		[]byte("test"),
		[]byte("test"),
		[]byte("apple"),
		[]byte("zoom"),
		[]byte("test_case"),
		[]byte("test_case_aBd"),
	}
	var cases1 = []string{
		"TestCase",
		"Test",
		"test",
		"Apple",
		"Zoom",
		"testCase",
		"test",
		"test",
		"apple",
		"zoom",
		"test_case",
		"test_case_aBd",
	}

	for i := 0; i < len(cases); i++ {
		assert.Equal(t, ToCamel(cases1[i]), bytesconv.B2s(ToCamelByte(cases[i])), "not equal")
	}
}
