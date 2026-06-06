package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LoadDotEnv reads a .env file and sets any key=value pairs as environment
// variables, skipping blank lines and comments.  Existing OS env vars always
// take precedence so that CI/CD secrets are never overridden.
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env is optional; missing file is not an error
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip surrounding single or double quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" { // don't override pre-existing env vars
			os.Setenv(key, val)
		}
	}
}

// DotEnvCandidates returns a prioritised list of paths to look for a .env file.
// This covers: running from the project root, running a binary from bin/, and
// running via `go run ./cmd/server` where the binary lands in a temp dir.
func DotEnvCandidates() []string {
	candidates := []string{".env"} // cwd — works for `go run` from project root

	// Walk up from the executable's location (covers bin/sit-validator → root).
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for range 4 { // up to 4 levels up
			candidates = append(candidates, filepath.Join(dir, ".env"))
			dir = filepath.Dir(dir)
		}
	}
	return candidates
}

// LoadFirstDotEnv tries each candidate path and loads the first existing .env.
func LoadFirstDotEnv() {
	for _, p := range DotEnvCandidates() {
		if _, err := os.Stat(p); err == nil {
			LoadDotEnv(p)
			break
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Feature flags
// ──────────────────────────────────────────────────────────────────────────────

// Features holds boolean toggles that gate optional functionality.
// Each flag defaults to true and can be disabled via the corresponding env var.
type Features struct {
	DriveEnabled bool // DRIVE_ENABLED (default true)
}

var (
	featOnce     sync.Once
	featInstance Features
)

// LoadFeatures reads feature-flag env vars and returns a Features struct.
// The result is cached after the first call.
func LoadFeatures() Features {
	featOnce.Do(func() {
		featInstance = Features{
			DriveEnabled: envBool("DRIVE_ENABLED", true),
		}
	})
	return featInstance
}

// envBool reads an environment variable as a boolean.  Returns fallback when
// the variable is unset or empty.  Truthy values: "1", "true", "yes".
func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}
