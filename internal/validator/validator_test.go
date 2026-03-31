package validator

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// --- shared test helpers ---------------------------------------------------

func testConfig() *config.Config {
	return &config.Config{
		Excel: config.ExcelConfig{
			SkipSheets:   []string{"Changelog", "Daftar Isi"},
			HeaderRow:    9,
			DataStartRow: 10,
			Metadata: []config.MetadataConfig{
				{Key: "provider_name", Row: 3, Col: 1},
				{Key: "partner_name", Row: 5, Col: 1},
				{Key: "test_date", Row: 6, Col: 1},
			},
			Columns: config.ColumnsConfig{
				No: 0, Service: 1, Scenario: 2,
				ExpectedResult: 3, Request: 4, Response: 5,
				Result: 6, Notes: 7,
			},
		},
		Validation: config.ValidationConfig{
			Request: config.FieldValidation{
				Required:            true,
				EmptySentinelValues: []string{"URL Endpoint:\n Header Request:\n Request Body:", ""},
				ErrorMessage:        "Request belum diisi",
			},
			Response: config.FieldValidation{
				Required:            true,
				EmptySentinelValues: []string{"Response Body:", ""},
				ErrorMessage:        "Response Body belum diisi",
			},
			Result: config.ResultValidation{
				Required:      true,
				AllowedValues: []string{"Passed", "Not Passed", "passed", "not passed", "PASSED", "NOT PASSED"},
				ErrorMessage:  "Result belum diisi",
			},
			Notes: config.NotesValidation{
				RequiredIfResult: "not passed",
				ErrorMessage:     "Notes wajib diisi jika Result adalah Not Passed",
			},
		},
	}
}

// buildXlsx creates an in-memory parser.File from a setup func.
func buildXlsx(t *testing.T, setup func(f *excelize.File)) *parser.File {
	t.Helper()
	f := excelize.NewFile()
	setup(f)
	var buf bytes.Buffer
	_, err := f.WriteTo(&buf)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	p, err := parser.FromReader(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	t.Cleanup(func() { p.Close() })
	return p
}

// cell returns an excelize cell name like "A10".
func cell(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// addOKRow writes a fully-complete test case row at the given 1-indexed rowNum.
func addOKRow(f *excelize.File, sheet string, rowNum int, no string) {
	vals := []interface{}{no, "Any Service", "Happy Path", "200 OK", "curl ...", `{"responseCode":"00"}`, "Passed", ""}
	for col, val := range vals {
		f.SetCellValue(sheet, cell(col+1, rowNum), val)
	}
}

// addIncompleteRow writes a row missing the request field.
func addIncompleteRow(f *excelize.File, sheet string, rowNum int, no string) {
	vals := []interface{}{no, "Any Service", "Error Path", "4xx", "", `{"responseCode":"01"}`, "Not Passed", "some note"}
	for col, val := range vals {
		f.SetCellValue(sheet, cell(col+1, rowNum), val)
	}
}

// setMeta writes provider/partner/date metadata values into column C.
// Config says label is in col B (col=1, 0-indexed), value one to the right → col C.
func setMeta(f *excelize.File, sheet, provider, partner, date string) {
	if provider != "" {
		f.SetCellValue(sheet, "C3", provider)
	}
	if partner != "" {
		f.SetCellValue(sheet, "C5", partner)
	}
	if date != "" {
		f.SetCellValue(sheet, "C6", date)
	}
}

// --- metadata tests --------------------------------------------------------

func TestValidateMetadata_AllPresent(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		setMeta(f, "ProductA", "PT Bank XYZ", "Partner Co.", "2025-03-01")
	})

	meta := validateMetadata(p, "ProductA", cfg.Excel.Metadata)

	assert.Equal(t, "ok", meta["provider_name"].Status)
	assert.Equal(t, "PT Bank XYZ", meta["provider_name"].Value)
	assert.Equal(t, "ok", meta["partner_name"].Status)
	assert.Equal(t, "Partner Co.", meta["partner_name"].Value)
	assert.Equal(t, "ok", meta["test_date"].Status)
	assert.Equal(t, "2025-03-01", meta["test_date"].Value)
}

func TestValidateMetadata_MissingPartner(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		setMeta(f, "ProductA", "PT Bank XYZ", "", "2025-03-01")
	})

	meta := validateMetadata(p, "ProductA", cfg.Excel.Metadata)

	assert.Equal(t, "ok", meta["provider_name"].Status)
	assert.Equal(t, "missing", meta["partner_name"].Status)
	assert.Equal(t, "", meta["partner_name"].Value)
	assert.Equal(t, "ok", meta["test_date"].Status)
}

func TestValidateMetadata_AllMissing(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
	})

	meta := validateMetadata(p, "ProductA", cfg.Excel.Metadata)

	for key, field := range meta {
		assert.Equal(t, "missing", field.Status, "expected missing for key %s", key)
	}
}

func TestValidateMetadata_WhitespaceOnlyTreatedAsMissing(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		f.SetCellValue("ProductA", "C3", "   ")
	})

	meta := validateMetadata(p, "ProductA", cfg.Excel.Metadata)
	assert.Equal(t, "missing", meta["provider_name"].Status)
}

// --- Validate (full pipeline) tests ----------------------------------------

func TestValidate_AllSheetsOK(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		for _, sheet := range []string{"ProductA", "ProductB"} {
			f.NewSheet(sheet)
			setMeta(f, sheet, "Bank", "Partner", "2025-01-01")
			addOKRow(f, sheet, 10, "1.1")
			addOKRow(f, sheet, 11, "1.2")
		}
	})

	report := Validate(p, cfg, []string{"ProductA", "ProductB"})

	assert.Equal(t, "ok", report.Status)
	assert.Equal(t, 2, report.Summary.TotalSheets)
	assert.Equal(t, 2, report.Summary.SheetsOK)
	assert.Equal(t, 4, report.Summary.TotalTestCases)
	assert.Equal(t, 4, report.Summary.TestCasesOK)
}

func TestValidate_IncompleteSheet(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		setMeta(f, "ProductA", "Bank", "Partner", "2025-01-01")
		addOKRow(f, "ProductA", 10, "1.1")
		addIncompleteRow(f, "ProductA", 11, "1.2")
	})

	report := Validate(p, cfg, []string{"ProductA"})

	require.Len(t, report.Sheets, 1)
	s := report.Sheets[0]
	assert.Equal(t, "ProductA", s.SheetName)
	assert.Equal(t, 2, s.Summary.Total)
	assert.Equal(t, 1, s.Summary.OK)
	assert.Equal(t, 1, s.Summary.Incomplete)
	assert.Equal(t, "incomplete", report.Status)
	assert.Equal(t, 1, report.Summary.SheetsIncomplete)
}

func TestValidate_SkipSheets(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("Changelog")
		f.NewSheet("Daftar Isi")
		f.NewSheet("ProductA")
		setMeta(f, "ProductA", "Bank", "Partner", "2025-01-01")
		addOKRow(f, "ProductA", 10, "1.1")
	})

	report := Validate(p, cfg, nil)

	for _, s := range report.Sheets {
		assert.NotEqual(t, "Changelog", s.SheetName)
		assert.NotEqual(t, "Daftar Isi", s.SheetName)
	}
}

func TestValidate_FilterSheets(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		f.NewSheet("ProductB")
		setMeta(f, "ProductA", "Bank", "Partner", "2025-01-01")
		addOKRow(f, "ProductA", 10, "1.1")
	})

	report := Validate(p, cfg, []string{"ProductA"})

	require.Len(t, report.Sheets, 1)
	assert.Equal(t, "ProductA", report.Sheets[0].SheetName)
}

func TestValidate_NoMatchingSheets(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
	})

	// Filter requests a sheet that doesn't exist
	report := Validate(p, cfg, []string{"NonExistent"})
	assert.Len(t, report.Sheets, 0)
}

func TestValidate_StopsAtEmptyNoColumn(t *testing.T) {
	cfg := testConfig()
	p := buildXlsx(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		setMeta(f, "ProductA", "Bank", "Partner", "2025-01-01")
		addOKRow(f, "ProductA", 10, "1.1")
		addOKRow(f, "ProductA", 11, "1.2")
		// Row 12 intentionally left empty → stop signal
		addOKRow(f, "ProductA", 13, "1.4") // must NOT be counted
	})

	report := Validate(p, cfg, []string{"ProductA"})

	require.Len(t, report.Sheets, 1)
	assert.Equal(t, 2, report.Sheets[0].Summary.Total, "should stop at first empty no column")
}

func TestValidate_ConcurrentSheets_CorrectResults(t *testing.T) {
	cfg := testConfig()
	// 8 sheets processed concurrently; the results slice must be correctly indexed
	sheets := make([]string, 8)
	for i := range sheets {
		sheets[i] = fmt.Sprintf("Product%d", i+1)
	}

	p := buildXlsx(t, func(f *excelize.File) {
		for _, name := range sheets {
			f.NewSheet(name)
			setMeta(f, name, "Bank", "Partner", "2025-01-01")
			addOKRow(f, name, 10, "1.1")
		}
	})

	report := Validate(p, cfg, sheets)

	assert.Len(t, report.Sheets, 8)
	totalOK := 0
	for _, s := range report.Sheets {
		totalOK += s.Summary.OK
	}
	assert.Equal(t, 8, totalOK)
}

// --- buildReport tests -----------------------------------------------------

func TestBuildReport_AllOK(t *testing.T) {
	sheets := []model.SheetReport{
		makeSheetReport("A", 5, 5),
		makeSheetReport("B", 3, 3),
	}
	report := buildReport(sheets)

	assert.Equal(t, "ok", report.Status)
	assert.Equal(t, 2, report.Summary.TotalSheets)
	assert.Equal(t, 2, report.Summary.SheetsOK)
	assert.Equal(t, 0, report.Summary.SheetsIncomplete)
	assert.Equal(t, 8, report.Summary.TotalTestCases)
	assert.Equal(t, 8, report.Summary.TestCasesOK)
	assert.Equal(t, 0, report.Summary.TestCasesIncomplete)
}

func TestBuildReport_SomeIncomplete(t *testing.T) {
	sheets := []model.SheetReport{
		makeSheetReport("A", 5, 3),
		makeSheetReport("B", 3, 3),
	}
	report := buildReport(sheets)

	assert.Equal(t, "incomplete", report.Status)
	assert.Equal(t, 1, report.Summary.SheetsIncomplete)
	assert.Equal(t, 1, report.Summary.SheetsOK)
	assert.Equal(t, 2, report.Summary.TestCasesIncomplete)
}

func TestBuildReport_Empty(t *testing.T) {
	report := buildReport(nil)
	assert.Equal(t, "ok", report.Status)
	assert.Equal(t, 0, report.Summary.TotalSheets)
}

// --- makeSet / inSlice tests -----------------------------------------------

func TestMakeSet(t *testing.T) {
	s := makeSet([]string{"a", "b", "c"})
	assert.True(t, s["a"])
	assert.True(t, s["b"])
	assert.False(t, s["d"])
	assert.Equal(t, 3, len(s))
}

func TestMakeSet_Empty(t *testing.T) {
	s := makeSet(nil)
	assert.Empty(t, s)
}

func TestInSlice_Found(t *testing.T) {
	assert.True(t, inSlice([]string{"x", "y"}, "x"))
	assert.True(t, inSlice([]string{"x", "y"}, "y"))
}

func TestInSlice_NotFound(t *testing.T) {
	assert.False(t, inSlice([]string{"x", "y"}, "z"))
	assert.False(t, inSlice(nil, "x"))
}

// --- buildSheetSummary tests -----------------------------------------------

func TestBuildSheetSummary(t *testing.T) {
	tcs := []model.TestCaseResult{
		{Status: "ok"},
		{Status: "ok"},
		{Status: "incomplete"},
	}
	s := buildSheetSummary(tcs)
	assert.Equal(t, 3, s.Total)
	assert.Equal(t, 2, s.OK)
	assert.Equal(t, 1, s.Incomplete)
}

func TestBuildSheetSummary_Empty(t *testing.T) {
	s := buildSheetSummary(nil)
	assert.Equal(t, 0, s.Total)
	assert.Equal(t, 0, s.OK)
	assert.Equal(t, 0, s.Incomplete)
}

// --- helpers ----------------------------------------------------------------

func makeSheetReport(name string, total, ok int) model.SheetReport {
	tcs := make([]model.TestCaseResult, total)
	for i := 0; i < ok; i++ {
		tcs[i] = model.TestCaseResult{Status: "ok", Issues: []string{}}
	}
	for i := ok; i < total; i++ {
		tcs[i] = model.TestCaseResult{Status: "incomplete", Issues: []string{"some issue"}}
	}
	incomplete := total - ok
	return model.SheetReport{
		SheetName: name,
		Summary:   model.SheetSummary{Total: total, OK: ok, Incomplete: incomplete},
		TestCases: tcs,
	}
}
