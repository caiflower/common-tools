package test

type StructService struct {
}

func (t *StructService) Test() string {
	return "testResponse"
}

type Param struct {
	Args string `json:"args"`
	Name string `json:"name"`
}

func (t *StructService) Test1(param Param) Param {
	return param
}

func (t *StructService) Test2(param *Param) *Param {
	return param
}
