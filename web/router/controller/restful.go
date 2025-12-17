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
)

const (
	pathParamReg = `\/\{[a-zA-Z][a-zA-Z0-9_]*[a-zA-Z]\}`
)

type RestfulController struct {
	method         string
	version        string
	group          string
	originPath     string
	controllerName string
	action         string
	targetMethod   *basic.Method
}

func NewRestFul() *RestfulController {
	return &RestfulController{}
}

func (c *RestfulController) Group(group string) *RestfulController {
	if !strings.HasPrefix(c.group, "/") {
		c.group = "/" + group
	}
	c.group = group
	return c
}

func (c *RestfulController) Version(version string) *RestfulController {
	if strings.HasPrefix(version, "/") {
		version = version[1:]
	}
	c.version = version
	return c
}

func (c *RestfulController) Controller(controllerName string) *RestfulController {
	c.controllerName = controllerName
	return c
}

func (c *RestfulController) Path(path string) *RestfulController {
	copyC := c.copyC()
	if !strings.HasPrefix(path, "/") {
		copyC.originPath = "/" + path
	} else {
		copyC.originPath = path
	}
	return copyC
}

func (c *RestfulController) Method(method string) *RestfulController {
	copyC := c.copyC()
	copyC.method = method
	return copyC
}

func (c *RestfulController) Action(action string) *RestfulController {
	copyC := c.copyC()
	copyC.action = action
	return copyC
}

func (c *RestfulController) TargetMethod(method *basic.Method) *RestfulController {
	copyC := c.copyC()
	if method == nil {
		panic("TargetMethod the method is nil")
	}
	copyC.targetMethod = method
	return copyC
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

func (c *RestfulController) GetTargetMethod() *basic.Method {
	return c.targetMethod
}

func (c *RestfulController) GetControllerName() string {
	return c.controllerName
}

func (c *RestfulController) GetOriginPath() string {
	return c.originPath
}

func (c *RestfulController) GetGroup() string {
	return c.group
}

func (c *RestfulController) copyC() *RestfulController {
	return &RestfulController{
		method:         c.method,
		version:        c.version,
		controllerName: c.controllerName,
		action:         c.action,
		targetMethod:   c.targetMethod,
		originPath:     c.originPath,
		group:          c.group,
	}
}
