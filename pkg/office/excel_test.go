package office

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestCreateExcel(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}

	excelName := dir + "/test.xlsx"

	spec := &ExcelSpec{
		FileName:  excelName,
		SheetName: "sheet",
		Titles:    []string{"测试名称", "测试年龄"},
		Data:      [][]interface{}{{"名称1", 1}, {"名称2", 2}, {"名称3", 3}},
	}

	err = CreateExcel(spec)
	if err != nil {
		panic(err)
	}

	fmt.Println(excelName)
}

func TestCreateMultiSheetExcel(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}

	excelName := dir + "/test.xlsx"

	spec := &MultiExcelSpec{
		FileName: excelName,
		Sheets: []*Sheet{
			{SheetName: "sh1", Titles: []string{"测试名称", "测试年龄"}, Data: [][]interface{}{{"名称1", 1}, {"名称2", 2}, {"名称3", 3}}},
			{Titles: []string{"测试名称2", "测试年龄2"}, Data: [][]interface{}{{"名称21", 21}, {"名称22", 22}, {"名称23", 23}}},
			{SheetName: "sh3", Titles: []string{"测试名称3", "测试年龄3"}, Data: [][]interface{}{{"名称31", 31}, {"名称32", 32}, {"名称33", 33}}}},
	}

	err = CreateMultiSheetExcel(spec)
	if err != nil {
		panic(err)
	}

	fmt.Println(excelName)
}
