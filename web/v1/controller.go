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

 package webv1

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
	"github.com/caiflower/common-tools/pkg/tools"
)

type controller struct {
	paths []string
	cls   *basic.Class
}

func newController(v interface{}, controllerRootPkgName, rootPath string) (*controller, error) {
	kind := reflect.TypeOf(v).Kind()
	switch kind {
	case reflect.Ptr, reflect.Interface:
		cls := basic.NewClass(v)
		path := cls.GetPath()
		if strings.Contains(path, controllerRootPkgName) {
			path = strings.Split(path, controllerRootPkgName)[1]
			if len(path) > 0 && strings.HasPrefix(path, "/") {
				path = path[1:]
			}
		} else {
			splits := strings.Split(path, "/")
			path = splits[len(splits)-1]
		}

		var paths []string
		paths = append(paths, cls.GetName())

		clsName := strings.Replace(cls.GetName(), ".", "/", 1)
		paths = append(paths, path+"/"+clsName)

		lowerClsName := strings.ToLower(clsName)
		paths = append(paths, path+"/"+lowerClsName)

		if strings.HasSuffix(lowerClsName, "service") {
			paths = append(paths, path+"/"+strings.Replace(lowerClsName, "service", "", 1))
		}

		if strings.HasSuffix(lowerClsName, "controller") {
			paths = append(paths, path+"/"+strings.Replace(lowerClsName, "controller", "", 1))
		}

		for i, _ := range paths {
			if i == 0 {
				continue
			}
			if !strings.HasPrefix(paths[i], "/") {
				paths[i] = "/" + paths[i]
			}
			if rootPath != "" {
				paths[i] = "/" + rootPath + paths[i]
			}
		}

		for _, method := range cls.GetAllMethod() {
			if method.HasArgs() {
				arg := method.GetArgs()[0]
				var argValue reflect.Value
				switch arg.Kind() {
				case reflect.Ptr:
					argValue = reflect.New(arg.Elem())
				case reflect.Struct:
					argValue = reflect.New(arg)
				default:
					panic(fmt.Sprintf("parse param failed. not support kind %s", arg.Kind()))
				}
				elem := reflect.TypeOf(argValue.Interface()).Elem()
				pkgPath := elem.PkgPath() + "." + elem.Name()
				if err := tools.DoTagFunc(argValue.Interface(), pkgPath, []func(reflect.StructField, reflect.Value, interface{}) error{buildValid}); err != nil {
					panic(err.Error())
				}
			}
		}

		return &controller{paths: paths, cls: cls}, nil
	default:
		return nil, fmt.Errorf("invalid type %s", kind)
	}
}
