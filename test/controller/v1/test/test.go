package test

import (
	"fmt"

	"github.com/caiflower/common-tools/web/e"
)

type StructService struct {
}

func (t *StructService) Test() string {
	return "testResponse"
}

type Param struct {
	Args string `json:"args"`
	Name string `json:"name"`
}

type Param2 struct {
	Args  string `json:"args"`
	Name  string
	Test  string    `param:"test"`
	Test1 []string  `param:"test1"`
	Test3 []float64 `param:"test3"`
	//UnSupport []Param   `param:"unSupportParam"`
}

func (t *StructService) Test1(param Param) Param {
	return param
}

func (t *StructService) Test2(param *Param) *Param {
	return param
}

func (t *StructService) Test3(param Param2) Param2 {
	return param
}

func (t *StructService) Test4(param Param) e.ApiError {
	return e.NewApiError(e.NotFound, "not found", nil)
}

func (t *StructService) Test5(param Param2) error {
	return fmt.Errorf("not found")
}

func (t *StructService) Test6(param Param2) error {
	panic("error")
}
