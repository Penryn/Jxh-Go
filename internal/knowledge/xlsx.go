package knowledge

import (
	"io"
	"strconv"

	"github.com/xuri/excelize/v2"
)

func ReadRowsFromXLSX(r io.Reader, sheet string) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if sheet == "" {
		sheet = "release"
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		if rowIsEmpty(row) {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func rowIsEmpty(row []string) bool {
	for _, cell := range row {
		if cell != "" {
			return false
		}
	}
	return true
}

func SyntheticSourceID(keyword string, rowNumber int) string {
	if keyword != "" {
		return normalizeKey(keyword)
	}
	return "row-" + strconv.Itoa(rowNumber)
}
