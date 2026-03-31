package model

// ValidationReport is the top-level response returned by POST /api/v1/validate.
type ValidationReport struct {
	Status  string        `json:"status"` // "ok" | "incomplete"
	Summary GlobalSummary `json:"summary"`
	Sheets  []SheetReport `json:"sheets"`
}

// GlobalSummary aggregates counts across all validated sheets.
type GlobalSummary struct {
	TotalSheets         int `json:"total_sheets"`
	SheetsOK            int `json:"sheets_ok"`
	SheetsIncomplete    int `json:"sheets_incomplete"`
	TotalTestCases      int `json:"total_test_cases"`
	TestCasesOK         int `json:"test_cases_ok"`
	TestCasesIncomplete int `json:"test_cases_incomplete"`
}

// SheetReport holds the validation result for a single product sheet.
type SheetReport struct {
	SheetName string               `json:"sheet_name"`
	Metadata  map[string]MetaField `json:"metadata"`
	Summary   SheetSummary         `json:"summary"`
	TestCases []TestCaseResult     `json:"test_cases"`
}

// MetaField represents a single metadata cell (provider name, partner name, test date).
type MetaField struct {
	Value  string `json:"value"`
	Status string `json:"status"` // "ok" | "missing"
}

// SheetSummary holds per-sheet test case counts.
type SheetSummary struct {
	Total      int `json:"total"`
	OK         int `json:"ok"`
	Incomplete int `json:"incomplete"`
}

// TestCaseResult holds the validation result for a single test case row.
type TestCaseResult struct {
	No       string   `json:"no"`
	Service  string   `json:"service"`
	Scenario string   `json:"scenario"`
	Status   string   `json:"status"` // "ok" | "incomplete"
	Issues   []string `json:"issues"`
}
