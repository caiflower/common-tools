package v1

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/caiflower/common-tools/pkg/basic"
)

type controller struct {
	path string
	cls  *basic.Class
}

func newController(v interface{}, rootPath string) (*controller, error) {
	kind := reflect.TypeOf(v).Kind()
	switch kind {
	case reflect.Pointer, reflect.Interface:
		cls := basic.NewClass(v)
		path := cls.GetPkgPath()
		if strings.Contains(path, rootPath) {
			path = strings.Split(path, rootPath)[1][1:]
		} else {
			splits := strings.Split(path, "/")
			path = splits[len(splits)-1]
		}
		return &controller{path: "/" + path, cls: cls}, nil
	default:
		return nil, fmt.Errorf("invalid type %s", kind)
	}
}
