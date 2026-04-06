package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalValidYAML returns a YAML string with all required fields set so that
// config.validate() passes; individual tests override what they need to test.
const minimalValidYAML = `
server:
  port: 9090
  max_upload_size_mb: 10
excel:
  skip_sheets:
    - Changelog
  header_row: 9
  data_start_row: 10
  metadata: []
  columns:
    no: 0
    service: 1
    scenario: 2
    expected_result: 3
    request: 4
    response: 5
    result: 6
    notes: 7
validation:
  request:
    required: true
    error_message: "fill request"
  response:
    required: true
    error_message: "fill response"
`

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "rules-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	f.Close()
	return f.Name()
}

func TestLoad_ValidFile(t *testing.T) {
	cfg, err := Load(writeYAML(t, minimalValidYAML))
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 10, cfg.Server.MaxUploadSizeMB)
	assert.Contains(t, cfg.Excel.SkipSheets, "Changelog")
	assert.Equal(t, 9, cfg.Excel.HeaderRow)
	assert.Equal(t, 10, cfg.Excel.DataStartRow)
}

func TestLoad_ServerDefaults(t *testing.T) {
	yaml := `
server:
  port: 0
  max_upload_size_mb: 0
excel:
  header_row: 9
  data_start_row: 10
  metadata: []
  columns:
    no: 0
    service: 1
    scenario: 2
    expected_result: 3
    request: 4
    response: 5
    result: 6
    notes: 7
validation:
  request:
    required: true
    error_message: "fill request"
  response:
    required: true
    error_message: "fill response"
`
	cfg, err := Load(writeYAML(t, yaml))
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 20, cfg.Server.MaxUploadSizeMB)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	assert.Error(t, err)
}

// --- T-4: validate() tests -----------------------------------------------

func TestLoad_ZeroHeaderRow_Fails(t *testing.T) {
	yaml := `
excel:
  header_row: 0
  data_start_row: 10
validation:
  request: {required: true, error_message: "x"}
  response: {required: true, error_message: "x"}
  result: {required: true, allowed_values: ["Passed"], error_message: "x", invalid_value_message: "x"}
  notes: {error_message: "x"}
`
	_, err := Load(writeYAML(t, yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "header_row")
}

func TestLoad_DataStartRowNotGreaterThanHeaderRow_Fails(t *testing.T) {
	yaml := `
excel:
  header_row: 9
  data_start_row: 9
validation:
  request: {required: true, error_message: "x"}
  response: {required: true, error_message: "x"}
  result: {required: true, allowed_values: ["Passed"], error_message: "x", invalid_value_message: "x"}
  notes: {error_message: "x"}
`
	_, err := Load(writeYAML(t, yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data_start_row")
}

func TestLoad_EmptyErrorMessage_Fails(t *testing.T) {
	yaml := `
excel:
  header_row: 9
  data_start_row: 10
validation:
  request: {required: true, error_message: ""}
  response: {required: true, error_message: "x"}
`
	_, err := Load(writeYAML(t, yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error_message")
}

func TestLoad_RequiredHeadersMissingErrorMessage_Fails(t *testing.T) {
	// required_headers set but required_header_error_message empty → must fail
	yaml := `
excel:
  header_row: 9
  data_start_row: 10
validation:
  request:
    required: true
    error_message: "fill request"
    required_headers: ["URL"]
    required_header_error_message: ""
  response: {required: true, error_message: "x"}
`
	_, err := Load(writeYAML(t, yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required_header_error_message")
}

func TestLoad_UniqueHeadersMissingErrorMessage_Fails(t *testing.T) {
	// unique_headers set but unique_header_error_message empty → must fail
	yaml := `
excel:
  header_row: 9
  data_start_row: 10
validation:
  request:
    required: true
    error_message: "fill request"
    required_header_error_message: "missing: %s"
    unique_headers: ["X-SIGNATURE"]
    unique_header_error_message: ""
  response: {required: true, error_message: "x"}
`
	_, err := Load(writeYAML(t, yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique_header_error_message")
}

// --- SheetOverrides / ValidationForSheet tests -------------------------------

func TestValidationForSheet_NoOverride_ReturnsGlobal(t *testing.T) {
	cfg, err := Load(writeYAML(t, minimalValidYAML))
	require.NoError(t, err)

	v := cfg.ValidationForSheet("AnySheet")
	assert.Equal(t, cfg.Validation, v)
}

func TestValidationForSheet_WithOverride_ReturnsOverride(t *testing.T) {
	yaml := minimalValidYAML + `
sheet_overrides:
  "QR MPM":
    validation:
      request:
        required: true
        error_message: "override request msg"
      response:
        required: true
        error_message: "override response msg"
`
	cfg, err := Load(writeYAML(t, yaml))
	require.NoError(t, err)

	v := cfg.ValidationForSheet("QR MPM")
	assert.Equal(t, "override request msg", v.Request.ErrorMessage)
	assert.Equal(t, "override response msg", v.Response.ErrorMessage)

	// Other sheets still get the global config.
	global := cfg.ValidationForSheet("Other Sheet")
	assert.Equal(t, cfg.Validation, global)
}

func TestValidationForSheet_NilValidation_ReturnsGlobal(t *testing.T) {
	// An override entry with no validation block → falls back to global.
	yaml := minimalValidYAML + `
sheet_overrides:
  "Empty Override": {}
`
	cfg, err := Load(writeYAML(t, yaml))
	require.NoError(t, err)

	v := cfg.ValidationForSheet("Empty Override")
	assert.Equal(t, cfg.Validation, v)
}

func TestLoad_SheetOverride_MissingRequestErrorMessage_Fails(t *testing.T) {
	yaml := minimalValidYAML + `
sheet_overrides:
  "Bad Sheet":
    validation:
      request:
        required: true
        error_message: ""
      response:
        required: true
        error_message: "x"
`
	_, err := Load(writeYAML(t, yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sheet_overrides")
	assert.Contains(t, err.Error(), "error_message")
}
