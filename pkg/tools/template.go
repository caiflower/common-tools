package tools

import (
	"bytes"
	"html/template"
)

func ExecTemplate(path string, dataMap map[string]interface{}) (res []byte, err error) {
	tt, err := template.ParseFiles(path)
	if err != nil {
		return
	}
	objBuff := &bytes.Buffer{}
	if err = tt.Execute(objBuff, dataMap); err != nil {
		return
	}

	res = objBuff.Bytes()
	return
}
