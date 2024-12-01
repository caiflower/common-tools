package pool

import (
	"fmt"
	"testing"
)

func Test_DoFuncString(t *testing.T) {
	slices := []string{"1", "2", "3", "4", "5"}
	fn := func(v interface{}) {
		fmt.Println(v)
	}
	err := DoFuncString(10, fn, slices...)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v", slices)
}

type Object struct {
	Age int
}

func Test_DoFuncInterface(t *testing.T) {
	var slices []*Object
	slices = append(slices, &Object{}, &Object{})
	fn := func(v interface{}) {
		object := v.(*Object)
		object.Age = 1
	}

	m := make(map[int]interface{})
	for i, v := range slices {
		m[i] = v
	}
	err := DoFuncInterface(10, fn, m)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v %v", slices[0], slices[1])
}
