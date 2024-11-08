package v1

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
)

type controller struct {
	paths []string
	cls   *basic.Class
}

func newController(v interface{}, rootPath string) (*controller, error) {
	kind := reflect.TypeOf(v).Kind()
	switch kind {
	case reflect.Pointer, reflect.Interface:
		cls := basic.NewClass(v)
		path := cls.GetPath()
		if strings.Contains(path, rootPath) {
			path = strings.Split(path, rootPath)[1][1:]
		} else {
			splits := strings.Split(path, "/")
			path = splits[len(splits)-1]
		}

		var paths []string
		paths = append(paths, cls.GetName())

		clsName := strings.Replace(cls.GetName(), ".", "/", 1)
		paths = append(paths, "/"+path+"/"+clsName)

		lowerClsName := strings.ToLower(clsName)
		paths = append(paths, "/"+path+"/"+lowerClsName)

		if strings.HasSuffix(lowerClsName, "service") {
			paths = append(paths, "/"+path+"/"+strings.Replace(lowerClsName, "service", "", 1))
		}

		if strings.HasSuffix(lowerClsName, "controller") {
			paths = append(paths, "/"+path+"/"+strings.Replace(lowerClsName, "controller", "", 1))
		}

		return &controller{paths: paths, cls: cls}, nil
	default:
		return nil, fmt.Errorf("invalid type %s", kind)
	}
}
