package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- test config & xlsx helpers --------------------------------------------

func testCfg() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Port: 8080, MaxUploadSizeMB: 20},
		Excel: config.ExcelConfig{
			SkipSheets:   []string{"Changelog"},
			HeaderRow:    9,
			DataStartRow: 10,
			Metadata: []config.MetadataConfig{
				{Key: "provider_name", Row: 3, Col: 1},
				{Key: "partner_name", Row: 5, Col: 1},
				{Key: "test_date", Row: 6, Col: 1},
			},
			Columns: config.ColumnsConfig{
				No: 0, Service: 1, Scenario: 2, ExpectedResult: 3,
				Request: 4, Response: 5, Result: 6, Notes: 7,
			},
		},
		Validation: config.ValidationConfig{
			Request: config.RequestValidation{
				Required:                   true,
				EmptySentinelValues:        []string{""},
				ErrorMessage:               "Request belum diisi",
				RequiredHeaders:            []string{"URL", "Content-Type", "Authorization", "X-SIGNATURE", "X-TIMESTAMP", "X-EXTERNAL-ID", "X-PARTNER-ID"},
				RequiredHeaderErrorMessage: "Header wajib tidak ditemukan di Request: %s",
				UniqueHeaders:              []string{"X-SIGNATURE", "X-TIMESTAMP"},
				UniqueHeaderErrorMessage:   "Nilai %s harus unik",
			},
			Response: config.ResponseValidation{
				Required:            true,
				EmptySentinelValues: []string{""},
				ErrorMessage:        "Response Body belum diisi",
				MatchExpectedResult: true,
				MatchErrorMessage:   "Response tidak sesuai dengan Expected Result",
				SuccessKeyword:      "Successful",
				SuccessMustContain:  "responseMessage",
				SuccessErrorMessage: "responseMessage wajib ada di Response",
			},
			Result: config.ResultValidation{Required: false},
			Notes:  config.NotesValidation{},
		},
	}
}

// buildXlsxBytes creates an in-memory xlsx and returns its raw bytes.
func buildXlsxBytes(t *testing.T, setup func(f *excelize.File)) []byte {
	t.Helper()
	f := excelize.NewFile()
	setup(f)
	var buf bytes.Buffer
	_, err := f.WriteTo(&buf)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return buf.Bytes()
}

// multipartBody creates a multipart/form-data body with the given file.
func multipartBody(t *testing.T, fieldName, filename string, content []byte) (io.Reader, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return &body, w.FormDataContentType()
}

// handlerRequest returns a fully-valid request string with unique signature/timestamp.
func handlerRequest(rowID string) string {
	return "URL: https://api.bri.co.id/v2/transfer\n" +
		"Content-Type: application/json\n" +
		"Authorization: Bearer token123\n" +
		"X-SIGNATURE: sig-" + rowID + "\n" +
		"X-TIMESTAMP: 2025-01-01T10:00:00+07:00-" + rowID + "\n" +
		"X-EXTERNAL-ID: ext-" + rowID + "\n" +
		"X-PARTNER-ID: partner-001"
}

// writeRow writes a fully-valid test case row at the given 1-indexed row in sheet.
func writeRow(f *excelize.File, sheet string, row int, no string) {
	vals := []interface{}{
		no, "Service", "Scenario", "Successful",
		handlerRequest(no),
		`{"responseCode":"2001600","responseMessage":"Successful"}`,
		"Passed", "",
	}
	colLetters := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	for i, col := range colLetters {
		f.SetCellValue(sheet, col+fmt.Sprint(row), vals[i])
	}
}

// setSheetMeta writes provider/partner/date into column C (the value column).
func setSheetMeta(f *excelize.File, sheet string) {
	f.SetCellValue(sheet, "C3", "PT Bank XYZ")
	f.SetCellValue(sheet, "C5", "Partner Co.")
	f.SetCellValue(sheet, "C6", "2025-03-01")
}

// sitXlsx builds a minimal SIT xlsx with one product sheet containing one OK row.
func sitXlsx(t *testing.T) []byte {
	return buildXlsxBytes(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		setSheetMeta(f, "ProductA")
		writeRow(f, "ProductA", 10, "1.1")
	})
}

// allSkippedXlsx builds an xlsx where every sheet is in the skip list.
// The default "Sheet1" created by excelize is renamed so it doesn't leak as a product sheet.
func allSkippedXlsx(t *testing.T) []byte {
	return buildXlsxBytes(t, func(f *excelize.File) {
		// Rename the default "Sheet1" to a skipped name so the file has no product sheets.
		f.SetSheetName("Sheet1", "Changelog")
	})
}

// perform sends an HTTP request to the test router and returns the response.
func perform(router http.Handler, method, url string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- GET /api/v1/health ----------------------------------------------------

func TestHealth_Returns200(t *testing.T) {
	router := setupRouter(testCfg())
	w := perform(router, "GET", "/api/v1/health", nil, "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, version, resp["version"])
}

// --- POST /api/v1/validate — error cases -----------------------------------

func TestValidate_NoFile_Returns400(t *testing.T) {
	router := setupRouter(testCfg())
	w := perform(router, "POST", "/api/v1/validate", nil, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file is required")
}

func TestValidate_NonXlsxExtension_Returns400(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "report.csv", []byte("not xlsx"))

	w := perform(router, "POST", "/api/v1/validate", body, ct)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid file format: expected .xlsx")
}

func TestValidate_CorruptXlsx_Returns422(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "broken.xlsx", []byte("this is not a valid xlsx"))

	w := perform(router, "POST", "/api/v1/validate", body, ct)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "cannot parse excel file")
}

func TestValidate_NoProductSheets_Returns422(t *testing.T) {
	router := setupRouter(testCfg())
	// The only sheet "Changelog" is in skip_sheets; no filter → validator sees 0 product sheets.
	xlsxData := allSkippedXlsx(t)
	body, ct := multipartBody(t, "file", "sit.xlsx", xlsxData)

	w := perform(router, "POST", "/api/v1/validate", body, ct)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "no recognizable product sheets found")
}

func TestValidate_InvalidSheetName_Returns400(t *testing.T) {
	router := setupRouter(testCfg())
	// sitXlsx has "ProductA"; request a sheet that does not exist.
	body, ct := multipartBody(t, "file", "sit.xlsx", sitXlsx(t))

	w := perform(router, "POST", "/api/v1/validate?sheets=NonExistent", body, ct)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "requested sheets not found in file", resp["error"])

	invalidSheets, ok := resp["invalid_sheets"].([]interface{})
	require.True(t, ok, "invalid_sheets should be an array")
	require.Len(t, invalidSheets, 1)
	assert.Equal(t, "NonExistent", invalidSheets[0])
}

func TestValidate_MultipleInvalidSheetNames_Returns400(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "sit.xlsx", sitXlsx(t))

	w := perform(router, "POST", "/api/v1/validate?sheets=Ghost1,Ghost2", body, ct)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	invalidSheets, ok := resp["invalid_sheets"].([]interface{})
	require.True(t, ok)
	assert.Len(t, invalidSheets, 2)
}

func TestValidate_SkippedSheetInFilter_Returns400(t *testing.T) {
	router := setupRouter(testCfg())
	// "Changelog" is in skip_sheets; requesting it should return 400, not 422.
	xlsxData := buildXlsxBytes(t, func(f *excelize.File) {
		f.NewSheet("Changelog")
		f.NewSheet("ProductA")
		setSheetMeta(f, "ProductA")
		writeRow(f, "ProductA", 10, "1.1")
	})
	body, ct := multipartBody(t, "file", "sit.xlsx", xlsxData)

	w := perform(router, "POST", "/api/v1/validate?sheets=Changelog", body, ct)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "requested sheets not found in file")
}

// --- POST /api/v1/validate — success (JSON) --------------------------------

func TestValidate_ValidFile_Returns200JSON(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "sit.xlsx", sitXlsx(t))

	w := perform(router, "POST", "/api/v1/validate?sheets=ProductA", body, ct)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var report model.ValidationReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))
	assert.Equal(t, "ok", report.Status)
	require.Len(t, report.Sheets, 1)
	assert.Equal(t, "ProductA", report.Sheets[0].SheetName)
	assert.Equal(t, 1, report.Summary.TotalTestCases)
	assert.Equal(t, 1, report.Summary.TestCasesOK)
}

func TestValidate_JSONReport_ContainsMetadata(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "sit.xlsx", sitXlsx(t))

	w := perform(router, "POST", "/api/v1/validate?sheets=ProductA", body, ct)
	require.Equal(t, http.StatusOK, w.Code)

	var report model.ValidationReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))

	meta := report.Sheets[0].Metadata
	assert.Equal(t, "ok", meta["provider_name"].Status)
	assert.Equal(t, "PT Bank XYZ", meta["provider_name"].Value)
	assert.Equal(t, "ok", meta["partner_name"].Status)
	assert.Equal(t, "ok", meta["test_date"].Status)
}

func TestValidate_IncompleteTestCase_ReportedCorrectly(t *testing.T) {
	router := setupRouter(testCfg())

	xlsxData := buildXlsxBytes(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		setSheetMeta(f, "ProductA")
		// Row 10: empty request → fails "Request belum diisi"
		vals := []interface{}{"1.1", "Service", "Scenario", "Successful", "", `{"responseCode":"2001600","responseMessage":"Successful"}`, "Passed", ""}
		colLetters := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
		for i, col := range colLetters {
			f.SetCellValue("ProductA", col+"10", vals[i])
		}
	})

	body, ct := multipartBody(t, "file", "sit.xlsx", xlsxData)
	w := perform(router, "POST", "/api/v1/validate?sheets=ProductA", body, ct)

	require.Equal(t, http.StatusOK, w.Code)

	var report model.ValidationReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))

	assert.Equal(t, "incomplete", report.Status)
	require.Len(t, report.Sheets[0].TestCases, 1)
	tc := report.Sheets[0].TestCases[0]
	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Request belum diisi")
}

func TestValidate_SheetFilterQueryParam(t *testing.T) {
	router := setupRouter(testCfg())
	xlsxData := buildXlsxBytes(t, func(f *excelize.File) {
		for _, sheet := range []string{"ProductA", "ProductB"} {
			f.NewSheet(sheet)
			setSheetMeta(f, sheet)
			writeRow(f, sheet, 10, "1.1")
		}
	})

	body, ct := multipartBody(t, "file", "sit.xlsx", xlsxData)
	w := perform(router, "POST", "/api/v1/validate?sheets=ProductA", body, ct)

	require.Equal(t, http.StatusOK, w.Code)
	var report model.ValidationReport
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &report))
	require.Len(t, report.Sheets, 1)
	assert.Equal(t, "ProductA", report.Sheets[0].SheetName)
}

// --- POST /api/v1/validate — Excel format ----------------------------------

func TestValidate_ExcelFormat_Returns200Xlsx(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "sit.xlsx", sitXlsx(t))

	w := perform(router, "POST", "/api/v1/validate?sheets=ProductA&format=excel", body, ct)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "sit_validation_report.xlsx")

	// The body must be a valid xlsx.
	ef, err := excelize.OpenReader(w.Body)
	require.NoError(t, err)
	defer ef.Close()
}

func TestValidate_ExcelFormat_ContainsAnnotations(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "sit.xlsx", sitXlsx(t))

	w := perform(router, "POST", "/api/v1/validate?sheets=ProductA&format=excel", body, ct)
	require.Equal(t, http.StatusOK, w.Code)

	ef, err := excelize.OpenReader(w.Body)
	require.NoError(t, err)
	defer ef.Close()

	// Header row (row 9, col I = 9)
	headerCell, _ := excelize.CoordinatesToCellName(9, 9)
	val, err := ef.GetCellValue("ProductA", headerCell)
	require.NoError(t, err)
	assert.Equal(t, "Validation Result", val)

	// Data row (row 10, col I = 9) should be "✓ OK"
	dataCell, _ := excelize.CoordinatesToCellName(9, 10)
	val, err = ef.GetCellValue("ProductA", dataCell)
	require.NoError(t, err)
	assert.Equal(t, "✓ OK", val)
}

// --- POST /api/v1/sheets ---------------------------------------------------

func TestSheets_NoFile_Returns400(t *testing.T) {
	router := setupRouter(testCfg())
	w := perform(router, "POST", "/api/v1/sheets", nil, "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "file is required")
}

func TestSheets_NonXlsxExtension_Returns400(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "report.csv", []byte("not xlsx"))

	w := perform(router, "POST", "/api/v1/sheets", body, ct)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid file format: expected .xlsx")
}

func TestSheets_CorruptXlsx_Returns422(t *testing.T) {
	router := setupRouter(testCfg())
	body, ct := multipartBody(t, "file", "broken.xlsx", []byte("not valid"))

	w := perform(router, "POST", "/api/v1/sheets", body, ct)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestSheets_ValidFile_ReturnsSheetList(t *testing.T) {
	router := setupRouter(testCfg())
	// sitXlsx has "Sheet1" (default) and "ProductA"
	xlsxData := buildXlsxBytes(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		f.NewSheet("ProductB")
		f.NewSheet("Changelog") // in skip_sheets — must be excluded
	})
	body, ct := multipartBody(t, "file", "sit.xlsx", xlsxData)

	w := perform(router, "POST", "/api/v1/sheets", body, ct)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string][]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	sheets := resp["sheets"]
	assert.Contains(t, sheets, "ProductA")
	assert.Contains(t, sheets, "ProductB")
	assert.NotContains(t, sheets, "Changelog")
}

// --- POST /api/v1/validate — sheets as form field --------------------------

func TestValidate_SheetFilterFormField(t *testing.T) {
	router := setupRouter(testCfg())
	xlsxData := buildXlsxBytes(t, func(f *excelize.File) {
		for _, sheet := range []string{"ProductA", "ProductB"} {
			f.NewSheet(sheet)
			setSheetMeta(f, sheet)
			writeRow(f, sheet, 10, "1.1")
		}
	})

	// Pass sheets as a form field, not a query string.
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "sit.xlsx")
	require.NoError(t, err)
	_, err = fw.Write(xlsxData)
	require.NoError(t, err)
	require.NoError(t, w.WriteField("sheets", "ProductA"))
	require.NoError(t, w.Close())

	resp := perform(router, "POST", "/api/v1/validate", &body, w.FormDataContentType())
	require.Equal(t, http.StatusOK, resp.Code)

	var report model.ValidationReport
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &report))
	require.Len(t, report.Sheets, 1)
	assert.Equal(t, "ProductA", report.Sheets[0].SheetName)
}

// --- request logger middleware ---------------------------------------------

func TestRequestLogger_SetsRequestID(t *testing.T) {
	router := setupRouter(testCfg())
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	req.Header.Set("X-Request-ID", "test-id-123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogger_GeneratesRequestIDIfMissing(t *testing.T) {
	router := setupRouter(testCfg())
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	// No X-Request-ID header set
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- T-1: file too large (413) ---------------------------------------------

func TestValidate_FileTooLarge_Returns413(t *testing.T) {
	// Set a 1-byte limit so even a tiny body triggers it.
	cfg := testCfg()
	cfg.Server.MaxUploadSizeMB = 1
	router := setupRouter(cfg)

	// Build a body that exceeds 1 MB.
	oversized := make([]byte, 1<<20+1) // 1 MiB + 1 byte
	body, ct := multipartBody(t, "file", "big.xlsx", oversized)

	w := perform(router, "POST", "/api/v1/validate", body, ct)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "file too large")
}
