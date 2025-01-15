package tools

import (
	"fmt"
	"testing"
)

func TestUUID(t *testing.T) {
	for i := 0; i < 100; i++ {
		fmt.Println(UUID())
	}
}

func TestGenerateId(t *testing.T) {
	m := make(map[string]struct{})

	for i := 0; i < 20000000; i++ {
		id := GenerateId("test")
		if _, ok := m[id]; ok {
			panic("id panic")
		}
		m[id] = struct{}{}
	}
}
