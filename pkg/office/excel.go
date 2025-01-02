package office

import (
	"errors"
	"strconv"

	"github.com/xuri/excelize/v2"
)

type MultiExcelSpec struct {
	FileName string
	Sheets   []*Sheet
}

type Sheet struct {
	SheetName string
	Titles    []string        // 每列的标题
	Data      [][]interface{} // 每行数据
}

type ExcelSpec struct {
	FileName  string          // 文件名称
	SheetName string          // Sheet名称
	Titles    []string        // 每列的标题
	Data      [][]interface{} // 每行数据
}

func CreateExcel(es *ExcelSpec) error {

	if es == nil || es.FileName == "" {
		return errors.New("parameter missing")
	}

	return CreateMultiSheetExcel(&MultiExcelSpec{
		FileName: es.FileName,
		Sheets:   []*Sheet{{SheetName: es.SheetName, Titles: es.Titles, Data: es.Data}},
	})
}

func CreateMultiSheetExcel(es *MultiExcelSpec) error {

	if es == nil || es.FileName == "" {
		return errors.New("parameter missing")
	}

	// 创建文件
	f := excelize.NewFile()

	for i, v := range es.Sheets {
		sheetName := "Sheet1"
		if i != 0 {
			sheetName = "Sheet" + strconv.Itoa(i+1)
			f.NewSheet(sheetName)
		}
		if v.SheetName != "" {
			f.SetSheetName(sheetName, v.SheetName)
			sheetName = v.SheetName
		}

		// 写标题
		if err := f.SetSheetRow(sheetName, "A1", &v.Titles); err != nil {
			return err
		}

		// 写内容
		row := 2
		for _, rowData := range v.Data {
			if err := f.SetSheetRow(sheetName, "A"+strconv.Itoa(row), &rowData); err != nil {
				return err
			}
			row++
		}
	}

	// 保存文件
	return f.SaveAs(es.FileName)
}
