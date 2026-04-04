package reporter

import (
	"bytes"
	"testing"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// --- helpers ----------------------------------------------------------------

func makeConfig() *config.Config {
	return &config.Config{
		Excel: config.ExcelConfig{
			HeaderRow:    9,
			DataStartRow: 10,
			Metadata: []config.MetadataConfig{
				{Key: "provider_name", Row: 3, Col: 1},
				{Key: "partner_name", Row: 5, Col: 1},
			},
		},
	}
}

func buildParserFile(t *testing.T, setup func(f *excelize.File)) *parser.File {
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

func sampleReport(sheetName string) model.ValidationReport {
	return model.ValidationReport{
		Status: "incomplete",
		Summary: model.GlobalSummary{
			TotalSheets: 1, SheetsOK: 0, SheetsIncomplete: 1,
			TotalTestCases: 2, TestCasesOK: 1, TestCasesIncomplete: 1,
		},
		Sheets: []model.SheetReport{
			{
				SheetName: sheetName,
				Metadata: map[string]model.MetaField{
					"provider_name": {Value: "PT Bank XYZ", Status: "ok"},
					"partner_name":  {Value: "", Status: "missing"},
				},
				Summary: model.SheetSummary{Total: 2, OK: 1, Incomplete: 1},
				TestCases: []model.TestCaseResult{
					{No: "1.1", Service: "Svc", Scenario: "Happy", Status: "ok", Issues: []string{}, RowNumber: 10},
					{No: "1.2", Service: "Svc", Scenario: "Error", Status: "incomplete",
						Issues: []string{"Request belum diisi", "Response Body belum diisi"}, RowNumber: 11},
				},
			},
		},
	}
}

// --- BuildJSON tests --------------------------------------------------------

func TestBuildJSON_PassThrough(t *testing.T) {
	in := sampleReport("Sheet1")
	out := BuildJSON(in)

	assert.Equal(t, in.Status, out.Status)
	assert.Equal(t, in.Summary, out.Summary)
	require.Len(t, out.Sheets, 1)
	assert.Equal(t, in.Sheets[0].SheetName, out.Sheets[0].SheetName)
	assert.Equal(t, in.Sheets[0].Summary, out.Sheets[0].Summary)
}

func TestBuildJSON_EmptyReport(t *testing.T) {
	in := model.ValidationReport{Status: "ok", Sheets: []model.SheetReport{}}
	out := BuildJSON(in)
	assert.Equal(t, "ok", out.Status)
	assert.Empty(t, out.Sheets)
}

// --- BuildExcel tests -------------------------------------------------------

func TestBuildExcel_ReturnsValidXlsx(t *testing.T) {
	cfg := makeConfig()
	p := buildParserFile(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
	})
	report := sampleReport("ProductA")

	data, err := BuildExcel(p, report, cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify the output is a parseable xlsx.
	ef, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer ef.Close()
}

func TestBuildExcel_WritesValidationResultHeader(t *testing.T) {
	cfg := makeConfig()
	p := buildParserFile(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
	})
	report := sampleReport("ProductA")

	data, err := BuildExcel(p, report, cfg)
	require.NoError(t, err)

	ef, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer ef.Close()

	// headerCol=9 (column I), headerRow=9
	headerCell, _ := excelize.CoordinatesToCellName(9, 9)
	val, err := ef.GetCellValue("ProductA", headerCell)
	require.NoError(t, err)
	assert.Equal(t, "Validation Result", val)
}

func TestBuildExcel_OKRowGetsCheckmark(t *testing.T) {
	cfg := makeConfig()
	p := buildParserFile(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
	})
	report := sampleReport("ProductA")

	data, err := BuildExcel(p, report, cfg)
	require.NoError(t, err)

	ef, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer ef.Close()

	// First test case (row 10, col I) should be "✓ OK"
	okCell, _ := excelize.CoordinatesToCellName(9, 10)
	val, err := ef.GetCellValue("ProductA", okCell)
	require.NoError(t, err)
	assert.Equal(t, "✓ OK", val)
}

func TestBuildExcel_IncompleteRowGetsIssues(t *testing.T) {
	cfg := makeConfig()
	p := buildParserFile(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
	})
	report := sampleReport("ProductA")

	data, err := BuildExcel(p, report, cfg)
	require.NoError(t, err)

	ef, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer ef.Close()

	// Second test case (row 11, col I) should contain issue text.
	issueCell, _ := excelize.CoordinatesToCellName(9, 11)
	val, err := ef.GetCellValue("ProductA", issueCell)
	require.NoError(t, err)
	assert.Contains(t, val, "Request belum diisi")
	assert.Contains(t, val, "Response Body belum diisi")
}

func TestBuildExcel_MultipleSheets(t *testing.T) {
	cfg := makeConfig()
	p := buildParserFile(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		f.NewSheet("ProductB")
	})

	report := model.ValidationReport{
		Sheets: []model.SheetReport{
			{SheetName: "ProductA", Metadata: map[string]model.MetaField{},
				TestCases: []model.TestCaseResult{
					{Status: "ok", Issues: []string{}, RowNumber: 10},
				}},
			{SheetName: "ProductB", Metadata: map[string]model.MetaField{},
				TestCases: []model.TestCaseResult{
					{Status: "incomplete", Issues: []string{"some issue"}, RowNumber: 10},
				}},
		},
	}

	data, err := BuildExcel(p, report, cfg)
	require.NoError(t, err)

	ef, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer ef.Close()

	okCell, _ := excelize.CoordinatesToCellName(9, 10)
	valA, err := ef.GetCellValue("ProductA", okCell)
	require.NoError(t, err)
	assert.Equal(t, "✓ OK", valA)

	valB, err := ef.GetCellValue("ProductB", okCell)
	require.NoError(t, err)
	assert.Equal(t, "some issue", valB)
}

// --- T-5: Row-mapping correctness when blank rows exist in original sheet ---

// TestBuildExcel_RowMappingWithGaps verifies that annotations land on the
// correct Excel row even when blank rows exist between test cases.
// Before I-2, the reporter used DataStartRow+i which would misalign; now it
// uses TestCaseResult.RowNumber so each annotation targets the exact source row.
func TestBuildExcel_RowMappingWithGaps(t *testing.T) {
	cfg := makeConfig()
	p := buildParserFile(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
	})

	// Row 10: first test case (ok)
	// Row 11: intentionally BLANK (gap)
	// Row 12: second test case (incomplete) — RowNumber=12, not 11
	report := model.ValidationReport{
		Sheets: []model.SheetReport{
			{
				SheetName: "ProductA",
				Metadata:  map[string]model.MetaField{},
				TestCases: []model.TestCaseResult{
					{Status: "ok", Issues: []string{}, RowNumber: 10},
					{Status: "incomplete", Issues: []string{"Request belum diisi"}, RowNumber: 12},
				},
			},
		},
	}

	data, err := BuildExcel(p, report, cfg)
	require.NoError(t, err)

	ef, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer ef.Close()

	// Row 10 col I → "✓ OK"
	okCell, _ := excelize.CoordinatesToCellName(9, 10)
	valOK, err := ef.GetCellValue("ProductA", okCell)
	require.NoError(t, err)
	assert.Equal(t, "✓ OK", valOK)

	// Row 11 col I → must be EMPTY (the gap row was not annotated)
	gapCell, _ := excelize.CoordinatesToCellName(9, 11)
	valGap, err := ef.GetCellValue("ProductA", gapCell)
	require.NoError(t, err)
	assert.Empty(t, valGap, "gap row must not be annotated")

	// Row 12 col I → issue text
	issueCell, _ := excelize.CoordinatesToCellName(9, 12)
	valIssue, err := ef.GetCellValue("ProductA", issueCell)
	require.NoError(t, err)
	assert.Contains(t, valIssue, "Request belum diisi")
}

func TestBuildExcel_MissingMetadataCellHighlighted(t *testing.T) {
	cfg := makeConfig()
	p := buildParserFile(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
	})
	// partner_name is "missing" in sampleReport
	report := sampleReport("ProductA")

	data, err := BuildExcel(p, report, cfg)
	require.NoError(t, err)

	ef, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer ef.Close()

	// partner_name: row=5, col=1 → value cell col = 1+2 = 3 (column C), row 5
	partnerCell, _ := excelize.CoordinatesToCellName(3, 5)
	style, err := ef.GetCellStyle("ProductA", partnerCell)
	require.NoError(t, err)
	// Style index > 0 means a style was applied (the red fill).
	assert.Greater(t, style, 0, "missing metadata cell should have a style applied")
}
