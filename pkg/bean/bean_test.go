package bean

import (
	"fmt"
	"testing"
)

type TestAutoWrite struct {
	TestAutoWrite1 *TestAutoWrite1 `autowrite:""`
	TestAutoWrite2 *TestAutoWrite2 `autowrite:""`
	//TestAutoWrite4 TestAutoWrite4 `autowrite:""`
}

type TestAutoWrite1 struct {
	Name int
}

type TestAutoWrite2 struct {
	TestAutoWrite3 *TestAutoWrite3 `autowrite:""`
}

type TestAutoWrite3 struct {
	TestAutoWrite2 *TestAutoWrite2 `autowrite:""`
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
