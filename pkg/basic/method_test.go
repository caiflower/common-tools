package basic

import (
	"fmt"
	"testing"
)

func TFunc() string {
	return "test1"
}

func TestNewFunc(t *testing.T) {
	method := NewMethod(nil, TFunc)
	invoke := method.Invoke(nil)
	fmt.Printf("invoke ret = %s\n", invoke[0].Interface())
}
