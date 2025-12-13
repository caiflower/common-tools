/*
 * Copyright 2024 caiflower Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controller

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
	c.originPath = path
	if tools.MatchReg(path, pathParamReg) {
		strList := tools.RegFind(path, pathParamReg)
		for _, str := range strList {
			c.pathParams = append(c.pathParams, str[2:len(str)-1])
		}
		clean := tools.RegReplace(path, pathParamReg, "")
		c.otherPaths = strings.Split(clean, "/")[1:]

		c.path = tools.RegReplace(path, pathParamReg, `/[a-zA-Z0-9_-]+`) + "/?$"
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

func (c *RestfulController) TargetMethod(method *basic.Method) *RestfulController {
	c.targetMethod = method
	return c
}

func (c *RestfulController) GetPath() string {
	return c.path
}

func (c *RestfulController) GetVersion() string {
	return c.version
}

func (c *RestfulController) GetMethod() string {
	return c.method
}

func (c *RestfulController) GetAction() string {
	return c.action
}

func (c *RestfulController) GetOtherPaths() []string {
	return c.otherPaths
}

func (c *RestfulController) GetTargetMethod() *basic.Method {
	return c.targetMethod
}

func (c *RestfulController) GetControllerName() string {
	return c.controllerName
}

func (c *RestfulController) GetOriginPath() string {
	return c.originPath
}

func (c *RestfulController) GetPathParams() []string {
	return c.pathParams
}
