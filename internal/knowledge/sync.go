package knowledge

import (
	"bytes"
	"context"
	"io"
)

type RowSource interface {
	Download(ctx context.Context) ([]byte, error)
}

type SyncResult struct {
	Entries []Entry
	Report  ImportReport
}

func ParseWorkbook(data []byte, sheet string) (SyncResult, error) {
	rows, err := ReadRowsFromXLSX(bytes.NewReader(data), sheet)
	if err != nil {
		return SyncResult{}, err
	}
	entries, report := ParseRows(rows)
	return SyncResult{Entries: entries, Report: report}, nil
}

func ParseWorkbookReader(r io.Reader, sheet string) (SyncResult, error) {
	rows, err := ReadRowsFromXLSX(r, sheet)
	if err != nil {
		return SyncResult{}, err
	}
	entries, report := ParseRows(rows)
	return SyncResult{Entries: entries, Report: report}, nil
}
