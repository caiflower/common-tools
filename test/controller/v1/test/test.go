package test

import (
	"fmt"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/web/e"
)

type StructService struct {
}

func (t *StructService) Test() string {
	return "testResponse"
}

type InnerHeader struct {
	RequestID string `header:"X-Request-Id"`
	UserID    string `header:"X-User-Id" verf:""`
}

type InnerParam struct {
	TestId      string `json:"testId" inList:"testId"`
	TestInt     []int  `json:"testInt" inList:"1,2,3,4,5" reg:"[1-3]+" between:"1,2" len:",1"`
	InnerHeader InnerHeader
}

type Param struct {
	RequestID         string `header:"X-Request-Id"`
	InnerStructHeader InnerHeader
	InnerPrtHeader    *InnerHeader
	TestId            string
	Args              string   `json:"args" param:"args" default:"testDefault"`
	Name              string   `json:"name"`
	Name1             *string  `verf:"nilable" len:",5"`
	MyName            []string `json:"myName" inList:"myName,myName1" reg:"[0-9a-zA-Z]+"`
	TestInt           []int    `json:"testInt" inList:"1,2,3,4,5" reg:"[1-3]+" between:"1,2"`
	InnerParam        *InnerParam
}

type Param2 struct {
	Args           string `json:"args" verf:""`
	Name           string
	Test           string    `param:"test"`
	Test1          []string  `param:"test1" verf:""`
	Test3          []float64 `param:"test3"`
	InnerPrtHeader *InnerHeader
	InnerParam     *InnerParam `verf:""`
	Time           basic.TimeStandard
	Time1          *basic.TimeStandard `verf:""`
	//UnSupport []Param   `param:"unSupportParam"`
}

func (t *StructService) Test1(param Param) Param {
	return param
}

func (t *StructService) Test2(param *Param) *Param {
	return param
}

func (t *StructService) Test3(param Param2) Param2 {
	fmt.Println(param.Time.UTCString())
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
