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
