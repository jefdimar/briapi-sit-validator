package validator

import (
	"strings"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
)

// validateTestCase evaluates a single data row against the validation rules and
// returns a TestCaseResult. The row slice is 0-indexed.
func validateTestCase(row []string, cols config.ColumnsConfig, vcfg config.ValidationConfig) model.TestCaseResult {
	get := func(idx int) string {
		if idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	result := model.TestCaseResult{
		No:       get(cols.No),
		Service:  get(cols.Service),
		Scenario: get(cols.Scenario),
		Issues:   []string{},
	}

	// 1. Validate Request column
	if vcfg.Request.Required {
		req := get(cols.Request)
		if isEmpty(req, vcfg.Request.EmptySentinelValues) {
			result.Issues = append(result.Issues, vcfg.Request.ErrorMessage)
		}
	}

	// 2. Validate Response column
	if vcfg.Response.Required {
		resp := get(cols.Response)
		if isEmpty(resp, vcfg.Response.EmptySentinelValues) {
			result.Issues = append(result.Issues, vcfg.Response.ErrorMessage)
		}
	}

	// 3. Validate Result column
	resultVal := get(cols.Result)
	if vcfg.Result.Required {
		if resultVal == "" {
			result.Issues = append(result.Issues, vcfg.Result.ErrorMessage)
		} else if !isAllowed(resultVal, vcfg.Result.AllowedValues) {
			result.Issues = append(result.Issues, vcfg.Result.InvalidValueMessage)
		}
	}

	// 4. Validate Notes (conditional on Result being "not passed")
	if strings.Contains(strings.ToLower(resultVal), "not passed") {
		notes := get(cols.Notes)
		if notes == "" {
			result.Issues = append(result.Issues, vcfg.Notes.ErrorMessage)
		}
	}

	if len(result.Issues) == 0 {
		result.Status = "ok"
	} else {
		result.Status = "incomplete"
	}

	return result
}

// isEmpty returns true if val is blank or matches any sentinel value.
// Both val and each sentinel are normalized (CRLF → LF) before comparison so
// that Windows Excel files (which store cell line-breaks as \r\n) match YAML
// sentinels that use \n.
func isEmpty(val string, sentinels []string) bool {
	normalized := strings.ReplaceAll(val, "\r\n", "\n")
	if normalized == "" {
		return true
	}
	for _, s := range sentinels {
		if normalized == strings.ReplaceAll(s, "\r\n", "\n") {
			return true
		}
	}
	return false
}

// isAllowed returns true if val (case-insensitive) matches any allowed value.
func isAllowed(val string, allowed []string) bool {
	lower := strings.ToLower(val)
	for _, a := range allowed {
		if strings.ToLower(a) == lower {
			return true
		}
	}
	return false
}
