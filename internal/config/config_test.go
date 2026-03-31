package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidFile(t *testing.T) {
	yaml := `
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
  result:
    required: true
    allowed_values: ["Passed", "Not Passed"]
    error_message: "fill result"
  notes:
    required_if_result: "not passed"
    error_message: "fill notes"
`
	f, err := os.CreateTemp(t.TempDir(), "rules-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(yaml)
	require.NoError(t, err)
	f.Close()

	cfg, err := Load(f.Name())
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 10, cfg.Server.MaxUploadSizeMB)
	assert.Contains(t, cfg.Excel.SkipSheets, "Changelog")
}

func TestLoad_Defaults(t *testing.T) {
	yaml := "server: {}\nexcel: {}\nvalidation: {}\n"
	f, err := os.CreateTemp(t.TempDir(), "rules-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(yaml)
	require.NoError(t, err)
	f.Close()

	cfg, err := Load(f.Name())
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 20, cfg.Server.MaxUploadSizeMB)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	assert.Error(t, err)
}
