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
			Required:      true,
			AllowedValues: []string{"Passed", "Not Passed", "passed", "not passed", "PASSED", "NOT PASSED"},
			ErrorMessage:  "Result belum diisi",
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
