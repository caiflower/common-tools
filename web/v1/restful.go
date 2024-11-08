package v1

import (
	"strings"

	"github.com/caiflower/common-tools/pkg/tools"
)

const (
	pathParamReg = `\/\{[a-zA-Z][a-zA-Z0-9_]*[a-zA-Z]\}`
)

type RestfulController struct {
	version    string
	path       string
	pathParams []string // 路经参数 /v1/products/{productId}/subProducts/{subProductId}, 那么pathParams = ["productId","subProductId"]
	otherPath  []string
}

func NewRestFul() *RestfulController {
	return &RestfulController{}
}

func (c *RestfulController) Version(version string) {
	c.version = version
}

func (c *RestfulController) Path(path string) {
	if tools.MatchReg(path, pathParamReg) {
		strList := tools.RegFind(path, pathParamReg)
		for _, str := range strList {
			c.pathParams = append(c.pathParams, str[2:len(str)-1])
		}
		clean := tools.RegReplace(path, pathParamReg, "")
		c.otherPath = strings.Split(clean, "/")[1:]

		c.path = tools.RegReplace(path, pathParamReg, `/([a-zA-Z0-9_\-\.\/])`)
	} else {
		c.path = path
	}
}

func (c *RestfulController) GetPath() string {
	return c.path
}

func (c *RestfulController) GetVersion() string {
	return c.version
}

func Register(controller *RestfulController) {

}
