package reporter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
	"github.com/xuri/excelize/v2"
)

const (
	colorGreen = "C6EFCE" // Excel-style light green
	colorRed   = "FFC7CE" // Excel-style light red
	headerCol  = 9        // Column I (1-indexed)
)

// BuildExcel annotates the original workbook with a "Validation Result" column
// per product sheet and returns the bytes of the resulting .xlsx file.
func BuildExcel(p *parser.File, report model.ValidationReport, cfg *config.Config) ([]byte, error) {
	f := p.Raw()

	// Build a quick lookup: sheet name → SheetReport
	sheetMap := make(map[string]model.SheetReport, len(report.Sheets))
	for _, s := range report.Sheets {
		sheetMap[s.SheetName] = s
	}

	greenStyle, err := buildStyle(f, colorGreen)
	if err != nil {
		return nil, fmt.Errorf("create green style: %w", err)
	}
	redStyle, err := buildStyle(f, colorRed)
	if err != nil {
		return nil, fmt.Errorf("create red style: %w", err)
	}

	for sheetName, sr := range sheetMap {
		headerCell, _ := excelize.CoordinatesToCellName(headerCol, cfg.Excel.HeaderRow)
		if err := f.SetCellValue(sheetName, headerCell, "Validation Result"); err != nil {
			return nil, fmt.Errorf("set header cell %s!%s: %w", sheetName, headerCell, err)
		}

		// Annotate metadata section: highlight missing fields.
		for _, m := range cfg.Excel.Metadata {
			if field, ok := sr.Metadata[m.Key]; ok && field.Status == "missing" {
				valCol := m.Col + 2
				cellName, _ := excelize.CoordinatesToCellName(valCol, m.Row)
				_ = f.SetCellStyle(sheetName, cellName, cellName, redStyle)
			}
		}

		// Annotate test case rows.
		for i, tc := range sr.TestCases {
			rowNum := cfg.Excel.DataStartRow + i
			cellName, _ := excelize.CoordinatesToCellName(headerCol, rowNum)

			var cellVal string
			var style int
			if tc.Status == "ok" {
				cellVal = "✓ OK"
				style = greenStyle
			} else {
				cellVal = strings.Join(tc.Issues, " | ")
				style = redStyle
			}

			if err := f.SetCellValue(sheetName, cellName, cellVal); err != nil {
				return nil, fmt.Errorf("set result cell %s!%s: %w", sheetName, cellName, err)
			}
			if err := f.SetCellStyle(sheetName, cellName, cellName, style); err != nil {
				return nil, fmt.Errorf("set style %s!%s: %w", sheetName, cellName, err)
			}
		}
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write annotated workbook: %w", err)
	}
	return buf.Bytes(), nil
}

func buildStyle(f *excelize.File, bgColor string) (int, error) {
	return f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{bgColor},
			Pattern: 1,
		},
	})
}
