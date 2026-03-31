package validator

import (
	"strings"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
)

// validateMetadata reads the metadata header rows for a sheet and returns a
// map of key → MetaField with status "ok" or "missing".
func validateMetadata(p *parser.File, sheet string, metaCfgs []config.MetadataConfig) map[string]model.MetaField {
	result := make(map[string]model.MetaField, len(metaCfgs))

	for _, m := range metaCfgs {
		// col in config is 0-indexed (A=0); excelize expects 1-indexed columns.
		// The value lives one column to the right of the label column.
		valCol := m.Col + 2 // +1 to convert 0→1-indexed, +1 to move right of label

		val, err := p.GetCellValue(sheet, m.Row, valCol)
		if err != nil {
			result[m.Key] = model.MetaField{Value: "", Status: "missing"}
			continue
		}

		val = strings.TrimSpace(val)
		if val == "" {
			result[m.Key] = model.MetaField{Value: "", Status: "missing"}
		} else {
			result[m.Key] = model.MetaField{Value: val, Status: "ok"}
		}
	}

	return result
}
