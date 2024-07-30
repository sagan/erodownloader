package util

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Convert "abc.xlsx" to "abc.xlsx.csv".
// Adopted from https://github.com/shenwei356/csvtk/blob/master/csvtk/cmd/xlsx2csv.go .
func Xlsx2Csv(xlsxFile string) error {
	xlsx, err := excelize.OpenFile(xlsxFile)
	if err != nil {
		return err
	}
	defer func() {
		// Close the spreadsheet.
		if err := xlsx.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	sheets := xlsx.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets")
	}
	csvFile, err := os.OpenFile(xlsxFile+".csv", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer csvFile.Close()
	writer := csv.NewWriter(csvFile)
	rows, err := xlsx.GetRows(sheets[0], excelize.Options{RawCellValue: true})
	if err != nil {
		return err
	}
	var nColsMax int = -1
	var nCols int
	for _, row := range rows {
		nCols = len(row)
		if nColsMax < nCols {
			nColsMax = nCols
		}
	}
	if nColsMax < 0 {
		nColsMax = 0
	}
	emptyRow := make([]string, nColsMax)
	var notBlank bool
	var data string
	var numEmptyRows int
	ignoreEmptyRow := true
	for i, row := range rows {
		if len(row) < nColsMax {
			row = append(row, emptyRow[0:nColsMax-len(row)]...)
		}
		if ignoreEmptyRow {
			notBlank = false
			for _, data = range row {
				if data != "" {
					notBlank = true
					break
				}
			}
			if !notBlank {
				numEmptyRows++
				continue
			}
		}
		// it's a ugly workaround. Excel store date / time as float. e.g. 2022-01-15 => 44576 .
		// However, excelize GetRows / GetValue... does not handle these values quite well.
		// If RawCellValue is false, date cell is readad as local string (e.g. 01-15-22 in en-US), which is unpredictable.
		// (For some reason, ShortDatePattern does NOT work in some cases)
		// If RawCellValue is true, date cell is readed as raw float.
		if i > 0 && len(row) <= len(rows[0]) {
			for j := range row {
				if !strings.Contains(rows[0][j], "日期") && !strings.Contains(strings.ToLower(rows[0][j]), "date") {
					continue
				}
				f, err := strconv.ParseFloat(row[j], 64)
				if err != nil {
					continue
				}
				t, err := excelize.ExcelDateToTime(f, false)
				if err != nil {
					continue
				}
				row[j] = t.Format("2006-01-02")
			}
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return nil
}
