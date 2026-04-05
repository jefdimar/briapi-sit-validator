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

// validRequest returns a request string that passes all mandatory-header checks.
func validRequest(sig, ts string) string {
	return "URL: https://api.bri.co.id/v2/transfer\n" +
		"Content-Type: application/json\n" +
		"Authorization: Bearer token123\n" +
		"X-SIGNATURE: " + sig + "\n" +
		"X-TIMESTAMP: " + ts + "\n" +
		"X-EXTERNAL-ID: ext-001\n" +
		"X-PARTNER-ID: partner-001\n" +
		"Body: {}"
}

func defaultValidation() config.ValidationConfig {
	return config.ValidationConfig{
		Request: config.RequestValidation{
			Required:                   true,
			EmptySentinelValues:        []string{"URL Endpoint:\n Header Request:\n Request Body:", ""},
			ErrorMessage:               "Request belum diisi",
			RequiredHeaders:            []string{"URL", "Content-Type", "Authorization", "X-SIGNATURE", "X-TIMESTAMP", "X-EXTERNAL-ID", "X-PARTNER-ID"},
			RequiredHeaderErrorMessage: "Header wajib tidak ditemukan di Request: %s",
			UniqueHeaders:              []string{"X-SIGNATURE", "X-TIMESTAMP"},
			UniqueHeaderErrorMessage:   "Nilai %s harus unik",
		},
		Response: config.ResponseValidation{
			Required:            true,
			EmptySentinelValues: []string{"Response Body:", ""},
			ErrorMessage:        "Response Body belum diisi",
			MatchExpectedResult: true,
			MatchErrorMessage:   "Response tidak sesuai dengan Expected Result",
			SuccessKeyword:      "Successful",
			SuccessMustContain:  "responseMessage",
			SuccessErrorMessage: "responseMessage wajib ada di Response jika Expected Result mengandung 'Successful'",
		},
		Result: config.ResultValidation{Required: false},
		Notes:  config.NotesValidation{},
	}
}

// A fully valid row: request has all headers, response matches expected result.
var okRow = []string{
	"8.1", "Any Service", "Happy Path", "Successful",
	validRequest("sig-abc", "2025-01-01T10:00:00+07:00"),
	`{"responseCode":"2001600","responseMessage":"Successful"}`,
	"Passed", "",
}

func TestValidateTestCase_OK(t *testing.T) {
	tc := validateTestCase(okRow, defaultCols(), defaultValidation())
	assert.Equal(t, "ok", tc.Status)
	assert.Empty(t, tc.Issues)
	assert.Equal(t, "8.1", tc.No)
}

// --- Rule: Request empty ---------------------------------------------------

func TestValidateTestCase_MissingRequest(t *testing.T) {
	row := []string{"8.2", "Service", "Scenario", "Successful", "", `{"responseCode":"00","responseMessage":"Successful"}`, "", ""}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Request belum diisi")
}

// --- Rule 3: required headers in Request -----------------------------------

func TestValidateTestCase_MissingRequiredHeader(t *testing.T) {
	// Request without X-SIGNATURE
	req := "URL: https://api.bri.co.id\n" +
		"Content-Type: application/json\n" +
		"Authorization: Bearer token\n" +
		"X-TIMESTAMP: 2025-01-01T10:00:00+07:00\n" +
		"X-EXTERNAL-ID: ext-001\n" +
		"X-PARTNER-ID: partner-001"
	row := []string{"8.3", "Service", "Scenario", "Successful", req, `{"responseMessage":"Successful"}`, "", ""}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Header wajib tidak ditemukan di Request: X-SIGNATURE")
}

func TestValidateTestCase_AllRequiredHeadersPresent(t *testing.T) {
	row := append([]string{}, okRow...)
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "ok", tc.Status)
	// No header-related issues
	for _, issue := range tc.Issues {
		assert.NotContains(t, issue, "Header wajib")
	}
}

// --- Rule 2: Response must match Expected Result ---------------------------

func TestValidateTestCase_ResponseMatchesExpectedResult(t *testing.T) {
	// Expected "Successful", response contains "Successful" → ok
	tc := validateTestCase(okRow, defaultCols(), defaultValidation())
	assert.Equal(t, "ok", tc.Status)
}

func TestValidateTestCase_ResponseDoesNotMatchExpectedResult(t *testing.T) {
	row := []string{
		"8.4", "Service", "Scenario", "Successful",
		validRequest("sig-xyz", "2025-01-01T11:00:00+07:00"),
		`{"responseCode":"4001601","responseMessage":"Invalid Token"}`,
		"", "",
	}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Response tidak sesuai dengan Expected Result")
}

func TestValidateTestCase_ExpectedResultEmpty_MatchSkipped(t *testing.T) {
	// Empty expected result → fuzzy match is skipped
	row := []string{
		"8.5", "Service", "Scenario", "",
		validRequest("sig-abc", "2025-01-01T10:00:00+07:00"),
		`{"responseCode":"2001600","responseMessage":"Successful"}`,
		"", "",
	}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "ok", tc.Status)
}

// --- Rule 5: responseMessage required when Expected Result has "Successful" --

func TestValidateTestCase_SuccessfulExpectedResult_MissingResponseMessage(t *testing.T) {
	row := []string{
		"8.6", "Service", "Scenario", "Successful",
		validRequest("sig-abc", "2025-01-01T10:00:00+07:00"),
		`{"responseCode":"2001600"}`, // no responseMessage
		"", "",
	}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "responseMessage wajib ada di Response jika Expected Result mengandung 'Successful'")
}

func TestValidateTestCase_SuccessfulExpectedResult_HasResponseMessage(t *testing.T) {
	tc := validateTestCase(okRow, defaultCols(), defaultValidation())
	assert.Equal(t, "ok", tc.Status)
}

func TestValidateTestCase_NonSuccessfulExpectedResult_NoResponseMessageRequired(t *testing.T) {
	row := []string{
		"8.7", "Service", "Scenario", "Invalid Token",
		validRequest("sig-abc", "2025-01-01T10:00:00+07:00"),
		`{"responseCode":"4001601","responseMessage":"Invalid Token"}`,
		"", "",
	}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "ok", tc.Status)
}

// --- Rule 6: Result and Notes are not validated ----------------------------

func TestValidateTestCase_ResultAndNotesIgnored(t *testing.T) {
	// Result is empty, Notes is empty — should still be "ok" since rule 6 disables them
	row := []string{
		"8.8", "Service", "Scenario", "Successful",
		validRequest("sig-abc", "2025-01-01T10:00:00+07:00"),
		`{"responseCode":"2001600","responseMessage":"Successful"}`,
		"", "", // empty Result and Notes
	}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "ok", tc.Status)
	assert.Empty(t, tc.Issues)
}

func TestValidateTestCase_InvalidResult_NotFlagged(t *testing.T) {
	row := []string{
		"8.9", "Service", "Scenario", "Successful",
		validRequest("sig-abc", "2025-01-01T10:00:00+07:00"),
		`{"responseCode":"2001600","responseMessage":"Successful"}`,
		"InvalidValue", "", // Result is invalid — should be ignored
	}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "ok", tc.Status)
}

// --- Response empty --------------------------------------------------------

func TestValidateTestCase_MissingResponse(t *testing.T) {
	row := []string{"8.10", "Service", "Scenario", "Successful", validRequest("sig-abc", "ts-abc"), "", "", ""}
	tc := validateTestCase(row, defaultCols(), defaultValidation())
	assert.Equal(t, "incomplete", tc.Status)
	assert.Contains(t, tc.Issues, "Response Body belum diisi")
}

// --- Helper function tests -------------------------------------------------

func TestIsEmpty(t *testing.T) {
	assert.True(t, isEmpty("", nil))
	assert.True(t, isEmpty("sentinel", []string{"sentinel"}))
	assert.False(t, isEmpty("value", []string{"sentinel"}))
}

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
		{"multiline LF sentinel", "URL Endpoint:\n Header Request:\n Request Body:", true},
		{"multiline CRLF sentinel", "URL Endpoint:\r\n Header Request:\r\n Request Body:", true},
		{"single-line sentinel", "URL Endpoint: Header Request: Request Body:", true},
		{"empty string", "", true},
		{"filled curl value", "curl -X POST https://api.example.com", false},
		{"contains but not equal", "Some prefix URL Endpoint:\n Header Request:\n Request Body:", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.empty, isEmpty(tc.val, sentinels))
		})
	}
}

func TestContainsHeader(t *testing.T) {
	req := "URL: https://api.example.com\nContent-Type: application/json\nX-SIGNATURE: abc123"
	assert.True(t, containsHeader(req, "URL"))
	assert.True(t, containsHeader(req, "Content-Type"))
	assert.True(t, containsHeader(req, "X-SIGNATURE"))
	assert.True(t, containsHeader(req, "x-signature")) // case-insensitive
	assert.False(t, containsHeader(req, "Authorization"))
	assert.False(t, containsHeader(req, "X-TIMESTAMP"))
}

func TestMatchesExpectedResult(t *testing.T) {
	// "Successful" in expected → "Successful" in response
	assert.True(t, matchesExpectedResult("Successful", `{"responseMessage":"Successful"}`))
	// "Invalid Token" in expected → "Invalid" in response
	assert.True(t, matchesExpectedResult("Invalid Token", `{"responseMessage":"Invalid Token"}`))
	// No overlap
	assert.False(t, matchesExpectedResult("Successful", `{"responseCode":"4001601","responseMessage":"Invalid"}`))
	// Empty expected result → always true (skip)
	assert.True(t, matchesExpectedResult("", `{}`))
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("200 OK")
	assert.Equal(t, []string{"200", "OK"}, tokens)
	tokens = tokenize("responseMessage:Successful")
	assert.Contains(t, tokens, "responseMessage")
	assert.Contains(t, tokens, "Successful")
}
