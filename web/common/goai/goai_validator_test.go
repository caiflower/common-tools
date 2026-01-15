package goai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type validateNested struct {
	Value string `json:"value" verf:"required"`
}

type validateTarget struct {
	Name     string           `json:"name" verf:"required|len:2,4"`
	Age      int              `json:"age" verf:"between:1,3"`
	Role     string           `json:"role" verf:"inList:admin,user"`
	Tags     []string         `json:"tags" verf:"itemLen:2,4"`
	Code     string           `json:"code" verf:"reg:^A\\d+$"`
	Optional *string          `json:"optional" verf:"nilable|len:1,3"`
	Nested   validateNested   `json:"nested"`
	Items    []validateNested `json:"items"`
}

func newValidTarget() validateTarget {
	return validateTarget{
		Name:     "Tom",
		Age:      2,
		Role:     "admin",
		Tags:     []string{"aa", "bbb"},
		Code:     "A123",
		Optional: nil,
		Nested:   validateNested{Value: "ok"},
		Items: []validateNested{
			{Value: "ok1"},
			{Value: "ok2"},
		},
	}
}

func TestValidate_Success(t *testing.T) {
	oai := New()
	target := newValidTarget()

	err := oai.Validate(&target)
	assert.NoError(t, err)
}

func TestValidate_Failures(t *testing.T) {
	t.Run("required string", func(t *testing.T) {
		oai := New()
		target := newValidTarget()
		target.Name = ""

		err := oai.Validate(&target)
		assert.Error(t, err)
		assert.Equal(t, "validateTarget.Name is missing", err.Error())
	})

	t.Run("between numeric range", func(t *testing.T) {
		oai := New()
		target := newValidTarget()
		target.Age = 5

		err := oai.Validate(&target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validateTarget.Age is not between 1 and 3")
	})

	t.Run("inList enum", func(t *testing.T) {
		oai := New()
		target := newValidTarget()
		target.Role = "guest"

		err := oai.Validate(&target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validateTarget.Role is not in")
	})

	t.Run("itemLen on slice", func(t *testing.T) {
		oai := New()
		target := newValidTarget()
		target.Tags = []string{"a"}

		err := oai.Validate(&target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validateTarget.Tags[0] len is less than 2")
	})

	t.Run("regex pattern", func(t *testing.T) {
		oai := New()
		target := newValidTarget()
		target.Code = "B123"

		err := oai.Validate(&target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validateTarget.Code is not match")
	})

	t.Run("nilable allows zero but validates non-zero value", func(t *testing.T) {
		oai := New()
		target := newValidTarget()
		// nil is allowed
		err := oai.Validate(&target)
		assert.NoError(t, err)

		// zero value (empty string) is also allowed for nilable
		empty := ""
		target.Optional = &empty
		err = oai.Validate(&target)
		assert.NoError(t, err)

		// non-zero value is validated against len rule
		long := "abcd"
		target.Optional = &long
		err = oai.Validate(&target)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validateTarget.Optional len is greater than 3")
	})

	t.Run("nested struct required", func(t *testing.T) {
		oai := New()
		target := newValidTarget()
		target.Nested = validateNested{}

		err := oai.Validate(&target)
		assert.Error(t, err)
		assert.Equal(t, "validateTarget.Nested.Value is missing", err.Error())
	})

	t.Run("slice of structs recursion", func(t *testing.T) {
		oai := New()
		target := newValidTarget()
		target.Items = []validateNested{
			{Value: "ok"},
			{},
		}

		err := oai.Validate(&target)
		assert.Error(t, err)
		assert.Equal(t, "validateTarget.Items[1].Value is missing", err.Error())
	})
}

type benchValidate struct {
	A1  string  `json:"a1" verf:"required|len:1,5"`
	A2  int     `json:"a2" verf:"between:0,10"`
	A3  string  `json:"a3"`
	A4  string  `json:"a4"`
	A5  string  `json:"a5"`
	A6  int     `json:"a6"`
	A7  int     `json:"a7"`
	A8  string  `json:"a8" verf:"reg:^ok$"`
	A9  string  `json:"a9"`
	A10 float64 `json:"a10"`
}

func newBenchValidate() benchValidate {
	return benchValidate{
		A1:  "ok",
		A2:  5,
		A3:  "x",
		A4:  "y",
		A5:  "z",
		A6:  1,
		A7:  2,
		A8:  "ok",
		A9:  "n",
		A10: 3.14,
	}
}

// 提前缓存索引、判断 verf 标签
func BenchmarkValidate_WithCache(b *testing.B) {
	oai := New()
	target := newBenchValidate()
	_ = oai.Validate(&target) // warm schema and cache

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := oai.Validate(&target); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidate_RecomputeIndexes(b *testing.B) {
	oai := New()
	target := newBenchValidate()
	_ = oai.Validate(&target) // warm schema once

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		oai.validator.mu.Lock()
		oai.validator.propIndexes = make(map[string][]int)
		oai.validator.mu.Unlock()

		if err := oai.Validate(&target); err != nil {
			b.Fatal(err)
		}
	}
}
