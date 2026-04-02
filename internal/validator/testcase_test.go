package validator

import (
	"testing"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/stretchr/testify/assert"
)

func defaultCols() config.ColumnsConfig {
	return config.ColumnsConfig{
		No:             0,
		Service:        1,
		Scenario:       2,
		ExpectedResult: 3,
		Request:        4,
		Response:       5,
		Result:         6,
		Notes:          7,
	}
}

func defaultValidation() config.ValidationConfig {
	return config.ValidationConfig{
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
			Required:            true,
			AllowedValues:       []string{"Passed", "Not Passed", "passed", "not passed", "PASSED", "NOT PASSED"},
			ErrorMessage:        "Result belum diisi",
			InvalidValueMessage: "Result tidak valid, harus Passed atau Not Passed",
		},
		Notes: config.NotesValidation{
			RequiredIfResult: "not passed",
			ErrorMessage:     "Notes wajib diisi jika Result adalah Not Passed",
		},
	}
}

func TestValidateTestCase_OK(t *testing.T) {
	row := []string{"8.1", "Any Service", "Happy Path", "200 OK", "curl ...", "{ \"responseCode\": \"00\" }", "Passed", ""}
	tc := validateTestCase(row, defaultCols(), defaultValidation())

	assert.Equal(t, "ok", tc.Status)
	assert.Empty(t, tc.Issues)
	assert.Equal(t, "8.1", tc.No)
}

func TestValidateTestCase_MissingRequest(t *testing.T) {
	row := []string{"8.2", "Service", "Scenario", "expected", "", "response body here", "Passed", ""}
	tc := validateTestCase(row, defaultCols(), defaultValidation())

	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Request belum diisi")
}

func TestValidateTestCase_MissingResponse(t *testing.T) {
	row := []string{"8.3", "Service", "Scenario", "expected", "curl ...", "", "Passed", ""}
	tc := validateTestCase(row, defaultCols(), defaultValidation())

	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Response Body belum diisi")
}

func TestValidateTestCase_InvalidResult(t *testing.T) {
	row := []string{"8.4", "Service", "Scenario", "expected", "curl ...", "response", "Belum", ""}
	tc := validateTestCase(row, defaultCols(), defaultValidation())

	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Result tidak valid, harus Passed atau Not Passed")
}

func TestValidateTestCase_NotPassedRequiresNotes(t *testing.T) {
	row := []string{"8.5", "Service", "Scenario", "expected", "curl ...", "response", "Not Passed", ""}
	tc := validateTestCase(row, defaultCols(), defaultValidation())

	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Notes wajib diisi jika Result adalah Not Passed")
}

func TestValidateTestCase_NotPassedWithNotes(t *testing.T) {
	row := []string{"8.6", "Service", "Scenario", "expected", "curl ...", "response", "Not Passed", "server returned 500"}
	tc := validateTestCase(row, defaultCols(), defaultValidation())

	assert.Equal(t, "ok", tc.Status)
	assert.Empty(t, tc.Issues)
}

func TestIsEmpty(t *testing.T) {
	assert.True(t, isEmpty("", nil))
	assert.True(t, isEmpty("sentinel", []string{"sentinel"}))
	assert.False(t, isEmpty("value", []string{"sentinel"}))
}

func TestIsAllowed(t *testing.T) {
	allowed := []string{"Passed", "Not Passed"}
	assert.True(t, isAllowed("passed", allowed))
	assert.True(t, isAllowed("NOT PASSED", allowed))
	assert.False(t, isAllowed("unknown", allowed))
}

// --- T-2: Sentinel value matching -----------------------------------------

// TestIsEmpty_SentinelVariants verifies all three sentinel patterns from
// rules.yaml are correctly matched (including \r\n normalisation for Windows).
func TestIsEmpty_SentinelVariants(t *testing.T) {
	sentinels := []string{
		"URL Endpoint:\n Header Request:\n Request Body:",
		"URL Endpoint: Header Request: Request Body:",
		"",
	}

	cases := []struct {
		name  string
		val   string
		empty bool
	}{
		// Exact match: LF newlines (Linux/Mac Excel)
		{"multiline LF sentinel", "URL Endpoint:\n Header Request:\n Request Body:", true},
		// CRLF variant — Windows Excel stores \r\n; should be normalised to \n before comparison
		{"multiline CRLF sentinel", "URL Endpoint:\r\n Header Request:\r\n Request Body:", true},
		// Space-separated variant (no newlines at all)
		{"single-line sentinel", "URL Endpoint: Header Request: Request Body:", true},
		// Empty string
		{"empty string", "", true},
		// Filled-in value must NOT match
		{"filled curl value", "curl -X POST https://api.example.com", false},
		// Value that contains a sentinel as substring but is not equal — must NOT match
		{"contains but not equal", "Some prefix URL Endpoint:\n Header Request:\n Request Body:", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.empty, isEmpty(tc.val, sentinels))
		})
	}
}

func TestIsEmpty_ResponseSentinels(t *testing.T) {
	sentinels := []string{"Response Body:", ""}
	assert.True(t, isEmpty("Response Body:", sentinels))
	assert.True(t, isEmpty("", sentinels))
	assert.False(t, isEmpty(`{"responseCode":"00"}`, sentinels))
}

// TestValidateTestCase_RequestAllSentinels runs the full test-case validation
// with each request sentinel and expects "incomplete" status each time.
func TestValidateTestCase_RequestAllSentinels(t *testing.T) {
	vcfg := defaultValidation()
	cols := defaultCols()

	sentinels := vcfg.Request.EmptySentinelValues
	// Also test the CRLF variant of the multiline sentinel.
	allInputs := append(sentinels, "URL Endpoint:\r\n Header Request:\r\n Request Body:")

	for _, sentinel := range allInputs {
		sentinel := sentinel
		t.Run("sentinel="+sentinel, func(t *testing.T) {
			row := []string{"1.1", "Svc", "Scenario", "200 OK", sentinel, `{"code":"00"}`, "Passed", ""}
			tc := validateTestCase(row, cols, vcfg)
			assert.Equal(t, "incomplete", tc.Status)
			assert.Contains(t, tc.Issues, vcfg.Request.ErrorMessage)
		})
	}
}

// --- T-3: Notes conditional — all result case variants --------------------

// TestValidateTestCase_NotesRequiredForAllNotPassedVariants checks that notes
// are required when result is any of the "not passed" case variants.
func TestValidateTestCase_NotesRequiredForAllNotPassedVariants(t *testing.T) {
	vcfg := defaultValidation()
	cols := defaultCols()
	notPassedVariants := []string{"Not Passed", "not passed", "NOT PASSED"}

	for _, result := range notPassedVariants {
		result := result
		t.Run("result="+result, func(t *testing.T) {
			row := []string{"1.1", "Svc", "Scenario", "200 OK", "curl ...", `{"code":"01"}`, result, ""}
			tc := validateTestCase(row, cols, vcfg)
			assert.Equal(t, "incomplete", tc.Status)
			assert.Contains(t, tc.Issues, vcfg.Notes.ErrorMessage,
				"notes should be required when result is %q", result)
		})
	}
}

// TestValidateTestCase_NotesNotRequiredForPassedVariants checks that notes are
// NOT required when result is any of the "passed" variants.
func TestValidateTestCase_NotesNotRequiredForPassedVariants(t *testing.T) {
	vcfg := defaultValidation()
	cols := defaultCols()
	passedVariants := []string{"Passed", "passed", "PASSED"}

	for _, result := range passedVariants {
		result := result
		t.Run("result="+result, func(t *testing.T) {
			row := []string{"1.1", "Svc", "Scenario", "200 OK", "curl ...", `{"code":"00"}`, result, ""}
			tc := validateTestCase(row, cols, vcfg)
			assert.Equal(t, "ok", tc.Status,
				"notes should NOT be required when result is %q", result)
			assert.NotContains(t, tc.Issues, vcfg.Notes.ErrorMessage)
		})
	}
}
