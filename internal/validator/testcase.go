package validator

import (
	"fmt"
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

	req := get(cols.Request)
	resp := get(cols.Response)
	expectedResult := get(cols.ExpectedResult)

	// Rule 3 + base: Validate Request column
	if vcfg.Request.Required {
		if isEmpty(req, vcfg.Request.EmptySentinelValues) {
			result.Issues = append(result.Issues, vcfg.Request.ErrorMessage)
		} else if len(vcfg.Request.RequiredHeaders) > 0 {
			// Rule 3: all mandatory HTTP headers must be present in the request text
			for _, header := range vcfg.Request.RequiredHeaders {
				if !containsHeader(req, header) {
					result.Issues = append(result.Issues,
						fmt.Sprintf(vcfg.Request.RequiredHeaderErrorMessage, header))
				}
			}
		}
	}

	// Rule 2 + base: Validate Response column
	if vcfg.Response.Required {
		if isEmpty(resp, vcfg.Response.EmptySentinelValues) {
			result.Issues = append(result.Issues, vcfg.Response.ErrorMessage)
		} else {
			// Rule 2: at least one keyword from Expected Result must appear in Response
			if vcfg.Response.MatchExpectedResult && expectedResult != "" {
				if !matchesExpectedResult(expectedResult, resp) {
					result.Issues = append(result.Issues, vcfg.Response.MatchErrorMessage)
				}
			}
			// Rule 5: if Expected Result contains the success keyword, responseMessage must appear in Response
			if vcfg.Response.SuccessKeyword != "" &&
				strings.Contains(strings.ToLower(expectedResult), strings.ToLower(vcfg.Response.SuccessKeyword)) {
				if !strings.Contains(strings.ToLower(resp), strings.ToLower(vcfg.Response.SuccessMustContain)) {
					result.Issues = append(result.Issues, vcfg.Response.SuccessErrorMessage)
				}
			}
		}
	}

	// Rule 6: Result and Notes columns are not validated.

	if len(result.Issues) == 0 {
		result.Status = "ok"
	} else {
		result.Status = "incomplete"
	}

	return result
}

// containsHeader reports whether the request text contains a given HTTP header
// key (case-insensitive, matching "<header>:" anywhere in the text).
func containsHeader(request, header string) bool {
	lower := strings.ToLower(strings.ReplaceAll(request, "\r\n", "\n"))
	return strings.Contains(lower, strings.ToLower(header)+":")
}

// matchesExpectedResult returns true if at least one alphanumeric token from
// expectedResult (case-insensitive, min length 2) appears as an exact token in
// response. Returns true when expectedResult is empty (nothing to match against).
func matchesExpectedResult(expectedResult, response string) bool {
	if expectedResult == "" {
		return true
	}
	expectedTokens := tokenize(expectedResult)
	responseSet := makeTokenSet(tokenize(response))
	for _, tok := range expectedTokens {
		if len(tok) >= 2 && responseSet[strings.ToLower(tok)] {
			return true
		}
	}
	return false
}

// tokenize splits s into alphanumeric tokens (letters, digits, hyphen, underscore).
func tokenize(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' ||
			'0' <= r && r <= '9' || r == '-' || r == '_')
	})
}

// makeTokenSet builds a lower-cased set from a token slice.
func makeTokenSet(tokens []string) map[string]bool {
	m := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		m[strings.ToLower(t)] = true
	}
	return m
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
// Kept for potential future use.
func isAllowed(val string, allowed []string) bool {
	lower := strings.ToLower(val)
	for _, a := range allowed {
		if strings.ToLower(a) == lower {
			return true
		}
	}
	return false
}
