package validator

import (
	"fmt"
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

	// Detect structural anomalies early; report them even if row parsing also fails.
	anomalies := detectSheetAnomalies(p, sheet, cfg, meta)
	if len(anomalies) > 0 {
		logger.Info("sheet anomalies detected", "anomalies", len(anomalies))
	}

	rows, err := p.GetRows(sheet)
	if err != nil {
		logger.Error("failed to read rows", "error", err, "row", 0)
		return model.SheetReport{
			SheetName: sheet,
			Anomalies: anomalies,
			Metadata:  meta,
			TestCases: []model.TestCaseResult{},
		}
	}

	// Use the sheet-specific validation config if one is defined; fall back to global.
	sheetValidation := cfg.ValidationForSheet(sheet)

	var testCases []model.TestCaseResult
	dataStart := cfg.Excel.DataStartRow - 1 // convert to 0-indexed

	// Rule 4: track header values per row for uniqueness checking.
	// headerValues[headerName][extractedValue] = []int{rowIndexInTestCases}
	headerValues := make(map[string]map[string][]int, len(sheetValidation.Request.UniqueHeaders))
	for _, h := range sheetValidation.Request.UniqueHeaders {
		headerValues[h] = make(map[string][]int)
	}

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

		tc := validateTestCase(row, cfg.Excel.Columns, sheetValidation)
		tc.RowNumber = rowIdx + 1 // store 1-indexed Excel row for reporter and logs

		// Collect header values for uniqueness check
		if cfg.Excel.Columns.Request < len(row) {
			req := strings.TrimSpace(row[cfg.Excel.Columns.Request])
			for header := range headerValues {
				if val := extractHeaderValue(req, header); val != "" {
					headerValues[header][val] = append(headerValues[header][val], len(testCases))
				}
			}
		}

		if tc.Status == "incomplete" {
			logger.Info("row incomplete", "row", tc.RowNumber, "no", tc.No, "issues", len(tc.Issues))
		} else {
			logger.Debug("row ok", "row", tc.RowNumber, "no", tc.No)
		}
		testCases = append(testCases, tc)
	}

	// Rule 4 post-processing: flag rows whose header value appears more than once.
	for header, valMap := range headerValues {
		for _, indices := range valMap {
			if len(indices) < 2 {
				continue
			}
			msg := fmt.Sprintf(sheetValidation.Request.UniqueHeaderErrorMessage, header)
			for _, idx := range indices {
				testCases[idx].Issues = append(testCases[idx].Issues, msg)
				testCases[idx].Status = "incomplete"
			}
		}
	}

	if testCases == nil {
		testCases = []model.TestCaseResult{}
	}

	summary := buildSheetSummary(testCases)

	return model.SheetReport{
		SheetName: sheet,
		Anomalies: anomalies,
		Metadata:  meta,
		Summary:   summary,
		TestCases: testCases,
	}
}

// extractHeaderValue extracts the value that follows "<headerName>:" on any line
// of the request text (case-insensitive). Returns "" if the header is absent.
func extractHeaderValue(request, headerName string) string {
	normalized := strings.ReplaceAll(request, "\r\n", "\n")
	prefix := strings.ToLower(headerName) + ":"
	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			return strings.TrimSpace(trimmed[len(headerName)+1:])
		}
	}
	return ""
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
		if s.Summary.Incomplete == 0 && len(s.Anomalies) == 0 {
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
