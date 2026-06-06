package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDotEnv(t *testing.T) {
	// Clean env first
	os.Unsetenv("TEST_KEY_1")
	os.Unsetenv("TEST_KEY_2")
	os.Unsetenv("TEST_KEY_3")

	content := `
# This is a comment
TEST_KEY_1=val1
TEST_KEY_2="val2 with spaces"
TEST_KEY_3='val3'
invalidline
`
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	err := os.WriteFile(envPath, []byte(content), 0644)
	require.NoError(t, err)

	LoadDotEnv(envPath)

	assert.Equal(t, "val1", os.Getenv("TEST_KEY_1"))
	assert.Equal(t, "val2 with spaces", os.Getenv("TEST_KEY_2"))
	assert.Equal(t, "val3", os.Getenv("TEST_KEY_3"))

	// Pre-existing env var takes precedence
	os.Setenv("TEST_KEY_1", "pre-existing")
	LoadDotEnv(envPath)
	assert.Equal(t, "pre-existing", os.Getenv("TEST_KEY_1"))

	// Clean up env vars
	os.Unsetenv("TEST_KEY_1")
	os.Unsetenv("TEST_KEY_2")
	os.Unsetenv("TEST_KEY_3")
}

func TestEnvBool(t *testing.T) {
	os.Unsetenv("BOOL_TEST")

	// Fallback
	assert.True(t, envBool("BOOL_TEST", true))
	assert.False(t, envBool("BOOL_TEST", false))

	// Truthy
	for _, val := range []string{"1", "true", "yes", "TRUE", "Yes"} {
		os.Setenv("BOOL_TEST", val)
		assert.True(t, envBool("BOOL_TEST", false))
	}

	// Falsy
	for _, val := range []string{"0", "false", "no", "FALSE", "No"} {
		os.Setenv("BOOL_TEST", val)
		assert.False(t, envBool("BOOL_TEST", true))
	}

	// Invalid fallback
	os.Setenv("BOOL_TEST", "invalid_value")
	assert.True(t, envBool("BOOL_TEST", true))
	assert.False(t, envBool("BOOL_TEST", false))

	os.Unsetenv("BOOL_TEST")
}

func TestDotEnvCandidates(t *testing.T) {
	candidates := DotEnvCandidates()
	assert.NotEmpty(t, candidates)
	assert.Equal(t, ".env", candidates[0])
}
