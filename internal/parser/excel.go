package parser

import (
	"fmt"
	"mime/multipart"

	"github.com/xuri/excelize/v2"
)

// File wraps an excelize.File and provides sheet-level accessors.
type File struct {
	f *excelize.File
}

// Open reads an xlsx file from a multipart file header.
func Open(fh *multipart.FileHeader) (*File, error) {
	src, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	ef, err := excelize.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("parse excel: %w", err)
	}

	return &File{f: ef}, nil
}

// SheetNames returns all sheet names in the workbook.
func (p *File) SheetNames() []string {
	return p.f.GetSheetList()
}

// GetCellValue returns the string value of a cell (1-indexed row and col).
func (p *File) GetCellValue(sheet string, row, col int) (string, error) {
	cellName, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return "", fmt.Errorf("coordinates (%d,%d): %w", col, row, err)
	}
	val, err := p.f.GetCellValue(sheet, cellName)
	if err != nil {
		return "", fmt.Errorf("get cell %s!%s: %w", sheet, cellName, err)
	}
	return val, nil
}

// GetRows returns all rows for a sheet as a 2D slice of strings.
func (p *File) GetRows(sheet string) ([][]string, error) {
	rows, err := p.f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("get rows for sheet %q: %w", sheet, err)
	}
	return rows, nil
}

// Raw returns the underlying excelize.File for use by the reporter.
func (p *File) Raw() *excelize.File {
	return p.f
}

// Close releases resources held by the underlying workbook.
func (p *File) Close() error {
	return p.f.Close()
}
