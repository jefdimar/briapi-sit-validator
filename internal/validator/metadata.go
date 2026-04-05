package validator

import (
	"strings"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
)

// validateMetadata reads the metadata header rows for a sheet and returns a
// map of key → MetaField with status "ok" or "missing".
//
// Rule 1: a metadata field is considered "ok" as long as there is any non-empty
// value after the colon. This handles two common Excel layouts:
//   - Label in column B, value in column C  →  reads column C directly.
//   - Label + value in the same (possibly merged) cell, e.g. "Label: Value"
//     →  reads column B and extracts the text after the first ':'.
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

		// Rule 1 fallback: if the dedicated value cell is empty, try reading the
		// label cell itself and extracting whatever follows the first ':'.
		if val == "" {
			labelCol := m.Col + 1 // 1-indexed label column
			if labelVal, lerr := p.GetCellValue(sheet, m.Row, labelCol); lerr == nil {
				val = extractAfterColon(strings.TrimSpace(labelVal))
			}
		}

		if val == "" {
			result[m.Key] = model.MetaField{Value: "", Status: "missing"}
		} else {
			result[m.Key] = model.MetaField{Value: val, Status: "ok"}
		}
	}

	return result
}

// extractAfterColon returns the trimmed substring after the first ':' in s.
// If s contains no colon the original string (trimmed) is returned.
func extractAfterColon(s string) string {
	idx := strings.Index(s, ":")
	if idx == -1 {
		return s
	}
	return strings.TrimSpace(s[idx+1:])
}
