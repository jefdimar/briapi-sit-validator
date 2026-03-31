package reporter

import (
	"github.com/jefdimar/briapi-sit-validator/internal/model"
)

// BuildJSON returns the ValidationReport as-is; Gin will marshal it to JSON.
// This thin wrapper exists so the handler doesn't import model directly and to
// allow future transformations (e.g., field filtering) without touching handler code.
func BuildJSON(report model.ValidationReport) model.ValidationReport {
	return report
}
