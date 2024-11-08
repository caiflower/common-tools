package v1

import (
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
)

const (
	pathParamReg = `\/\{[a-zA-Z][a-zA-Z0-9_]*[a-zA-Z]\}`
)

type RestfulController struct {
	method         string
	version        string
	originPath     string
	path           string
	controllerName string
	action         string
	pathParams     []string // 路经参数 /v1/products/{productId}/subProducts/{subProductId}, 那么pathParams = ["productId","subProductId"]
	otherPaths     []string
	targetMethod   *basic.Method
}

func NewRestFul() *RestfulController {
	return &RestfulController{}
}

func (c *RestfulController) Version(version string) *RestfulController {
	c.version = version
	return c
}

func (c *RestfulController) Path(path string) *RestfulController {
	if tools.MatchReg(path, pathParamReg) {
		c.originPath = path
		strList := tools.RegFind(path, pathParamReg)
		for _, str := range strList {
			c.pathParams = append(c.pathParams, str[2:len(str)-1])
		}
		clean := tools.RegReplace(path, pathParamReg, "")
		c.otherPaths = strings.Split(clean, "/")[1:]

		c.path = tools.RegReplace(path, pathParamReg, `/([a-zA-Z0-9_-]+/?)`)
	} else {
		c.path = path
	}
	return c
}

func (c *RestfulController) Method(method string) *RestfulController {
	c.method = method
	return c
}

func (c *RestfulController) Controller(controllerName string) *RestfulController {
	c.controllerName = controllerName
	return c
}

func (c *RestfulController) Action(action string) *RestfulController {
	c.action = action
	return c
}

func (c *RestfulController) GetPath() string {
	return c.path
}

func (c *RestfulController) GetVersion() string {
	return c.version
}
