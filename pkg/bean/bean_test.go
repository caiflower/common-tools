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
