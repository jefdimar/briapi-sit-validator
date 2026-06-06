package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Reload(t *testing.T) {
	yaml1 := minimalValidYAML // port 9090
	yaml2 := `
server:
  port: 9999
  max_upload_size_mb: 15
excel:
  skip_sheets: []
  header_row: 9
  data_start_row: 10
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
  request: {required: true, error_message: "x"}
  response: {required: true, error_message: "x"}
`
	tmpFile := writeYAML(t, yaml1)
	defer os.Remove(tmpFile)

	mgr, err := NewManager(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, 9090, mgr.Get().Server.Port)
	assert.Equal(t, tmpFile, mgr.Path())

	// Write new config and reload
	err = os.WriteFile(tmpFile, []byte(yaml2), 0644)
	require.NoError(t, err)

	err = mgr.Reload()
	require.NoError(t, err)
	assert.Equal(t, 9999, mgr.Get().Server.Port)
	assert.Equal(t, 15, mgr.Get().Server.MaxUploadSizeMB)
}
