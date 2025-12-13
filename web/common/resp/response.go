package resp

import "github.com/caiflower/common-tools/web/common/e"

type Result struct {
	RequestId string
	Data      interface{} `json:",omitempty"`
	Error     *e.Error    `json:",omitempty"`
}
