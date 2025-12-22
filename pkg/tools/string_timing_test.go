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
