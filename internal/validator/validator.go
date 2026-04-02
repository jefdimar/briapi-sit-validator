package validator

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
)

// Validate runs the full validation pipeline against the parsed Excel file.
// requestID is threaded into all log lines emitted during this request.
// It processes each product sheet concurrently and returns a ValidationReport.
func Validate(p *parser.File, cfg *config.Config, filterSheets []string, requestID string) model.ValidationReport {
	allSheets := p.SheetNames()
	skipSet := makeSet(cfg.Excel.SkipSheets)

	var productSheets []string
	for _, s := range allSheets {
		if skipSet[s] {
			continue
		}
		if len(filterSheets) > 0 && !inSlice(filterSheets, s) {
			continue
		}
		productSheets = append(productSheets, s)
	}

	results := make([]model.SheetReport, len(productSheets))
	var wg sync.WaitGroup

	for i, sheet := range productSheets {
		wg.Add(1)
		go func(idx int, sheetName string) {
			defer wg.Done()
			results[idx] = validateSheet(p, sheetName, cfg, requestID)
		}(i, sheet)
	}
	wg.Wait()

	return buildReport(results)
}

// validateSheet validates all metadata and test case rows within a single sheet.
func validateSheet(p *parser.File, sheet string, cfg *config.Config, requestID string) model.SheetReport {
	logger := slog.With("request_id", requestID, "sheet", sheet)

	meta := validateMetadata(p, sheet, cfg.Excel.Metadata)

	rows, err := p.GetRows(sheet)
	if err != nil {
		logger.Error("failed to read rows", "error", err, "row", 0)
		return model.SheetReport{
			SheetName: sheet,
			Metadata:  meta,
			TestCases: []model.TestCaseResult{},
		}
	}

	var testCases []model.TestCaseResult
	dataStart := cfg.Excel.DataStartRow - 1 // convert to 0-indexed

	for rowIdx := dataStart; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]

		// Stop when the "no" column is empty — end of test cases.
		noVal := ""
		if cfg.Excel.Columns.No < len(row) {
			noVal = strings.TrimSpace(row[cfg.Excel.Columns.No])
		}
		if noVal == "" {
			break
		}

		tc := validateTestCase(row, cfg.Excel.Columns, cfg.Validation)
		tc.RowNumber = rowIdx + 1 // store 1-indexed Excel row for reporter and logs
		if tc.Status == "incomplete" {
			logger.Info("row incomplete", "row", tc.RowNumber, "no", tc.No, "issues", len(tc.Issues))
		} else {
			logger.Debug("row ok", "row", tc.RowNumber, "no", tc.No)
		}
		testCases = append(testCases, tc)
	}

	if testCases == nil {
		testCases = []model.TestCaseResult{}
	}

	summary := buildSheetSummary(testCases)

	return model.SheetReport{
		SheetName: sheet,
		Metadata:  meta,
		Summary:   summary,
		TestCases: testCases,
	}
}

func buildSheetSummary(tcs []model.TestCaseResult) model.SheetSummary {
	s := model.SheetSummary{Total: len(tcs)}
	for _, tc := range tcs {
		if tc.Status == "ok" {
			s.OK++
		} else {
			s.Incomplete++
		}
	}
	return s
}

func buildReport(sheets []model.SheetReport) model.ValidationReport {
	var g model.GlobalSummary
	g.TotalSheets = len(sheets)

	for _, s := range sheets {
		if s.Summary.Incomplete == 0 {
			g.SheetsOK++
		} else {
			g.SheetsIncomplete++
		}
		g.TotalTestCases += s.Summary.Total
		g.TestCasesOK += s.Summary.OK
		g.TestCasesIncomplete += s.Summary.Incomplete
	}

	status := "ok"
	if g.SheetsIncomplete > 0 || g.TestCasesIncomplete > 0 {
		status = "incomplete"
	}

	return model.ValidationReport{
		Status:  status,
		Summary: g,
		Sheets:  sheets,
	}
}

func makeSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func inSlice(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
