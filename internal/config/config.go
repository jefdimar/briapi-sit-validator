package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration loaded from rules.yaml.
type Config struct {
	Server         ServerConfig                `yaml:"server"`
	Excel          ExcelConfig                 `yaml:"excel"`
	Validation     ValidationConfig            `yaml:"validation"`
	SheetOverrides map[string]SheetOverride    `yaml:"sheet_overrides"`
}

// SheetOverride allows a specific sheet to use a different ValidationConfig
// instead of the global one. Only the Validation field is overridable.
type SheetOverride struct {
	Validation *ValidationConfig `yaml:"validation,omitempty"`
}

// ValidationForSheet returns the effective ValidationConfig for the named sheet.
// If a sheet-specific override is defined in SheetOverrides, it is returned in full;
// otherwise the global Validation config is used as a fallback.
func (c *Config) ValidationForSheet(sheetName string) ValidationConfig {
	if c.SheetOverrides != nil {
		if override, ok := c.SheetOverrides[sheetName]; ok && override.Validation != nil {
			return *override.Validation
		}
	}
	return c.Validation
}

type ServerConfig struct {
	Port            int `yaml:"port"`
	MaxUploadSizeMB int `yaml:"max_upload_size_mb"`
}

type ExcelConfig struct {
	SkipSheets   []string         `yaml:"skip_sheets"`
	HeaderRow    int              `yaml:"header_row"`
	DataStartRow int              `yaml:"data_start_row"`
	Metadata     []MetadataConfig `yaml:"metadata"`
	Columns      ColumnsConfig    `yaml:"columns"`
}

type MetadataConfig struct {
	Key   string `yaml:"key"`
	Label string `yaml:"label"`
	Row   int    `yaml:"row"`
	Col   int    `yaml:"col"`
}

type ColumnsConfig struct {
	No             int `yaml:"no"`
	Service        int `yaml:"service"`
	Scenario       int `yaml:"scenario"`
	ExpectedResult int `yaml:"expected_result"`
	Request        int `yaml:"request"`
	Response       int `yaml:"response"`
	Result         int `yaml:"result"`
	Notes          int `yaml:"notes"`
}

type ValidationConfig struct {
	Request  RequestValidation  `yaml:"request"`
	Response ResponseValidation `yaml:"response"`
	Result   ResultValidation   `yaml:"result"`
	Notes    NotesValidation    `yaml:"notes"`
}

// RequestValidation holds rules for the Request column.
type RequestValidation struct {
	Required            bool     `yaml:"required"`
	EmptySentinelValues []string `yaml:"empty_sentinel_values"`
	ErrorMessage        string   `yaml:"error_message"`
	// Rule 3: every non-empty Request must contain all these header keys.
	RequiredHeaders            []string `yaml:"required_headers"`
	RequiredHeaderErrorMessage string   `yaml:"required_header_error_message"`
	// Rule 4: these header values must be unique across all rows in a sheet.
	UniqueHeaders            []string `yaml:"unique_headers"`
	UniqueHeaderErrorMessage string   `yaml:"unique_header_error_message"`
}

// ResponseValidation holds rules for the Response column.
type ResponseValidation struct {
	Required            bool     `yaml:"required"`
	EmptySentinelValues []string `yaml:"empty_sentinel_values"`
	ErrorMessage        string   `yaml:"error_message"`
	// Rule 2: at least one keyword from Expected Result must appear in Response.
	MatchExpectedResult bool   `yaml:"match_expected_result"`
	MatchErrorMessage   string `yaml:"match_error_message"`
	// Rule 5: if Expected Result contains SuccessKeyword, Response must include SuccessMustContain.
	SuccessKeyword      string `yaml:"success_keyword"`
	SuccessMustContain  string `yaml:"success_must_contain"`
	SuccessErrorMessage string `yaml:"success_error_message"`
}

// ResultValidation is kept for backwards-compatible YAML parsing; validation is
// disabled when Required is false (rule 6).
type ResultValidation struct {
	Required            bool     `yaml:"required"`
	AllowedValues       []string `yaml:"allowed_values"`
	ErrorMessage        string   `yaml:"error_message"`
	InvalidValueMessage string   `yaml:"invalid_value_message"`
}

// NotesValidation is kept for backwards-compatible YAML parsing; validation is
// disabled when RequiredIfResult is empty (rule 6).
type NotesValidation struct {
	RequiredIfResult string `yaml:"required_if_result"`
	ErrorMessage     string `yaml:"error_message"`
}

// Load reads and parses a YAML config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.MaxUploadSizeMB == 0 {
		cfg.Server.MaxUploadSizeMB = 20
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// validate checks that all required config fields are set to sensible values.
func (c *Config) validate() error {
	if c.Excel.HeaderRow <= 0 {
		return errors.New("excel.header_row must be > 0")
	}
	if c.Excel.DataStartRow <= 0 {
		return errors.New("excel.data_start_row must be > 0")
	}
	if c.Excel.DataStartRow <= c.Excel.HeaderRow {
		return errors.New("excel.data_start_row must be greater than excel.header_row")
	}
	if c.Validation.Request.Required && strings.TrimSpace(c.Validation.Request.ErrorMessage) == "" {
		return errors.New("validation.request.error_message must not be empty when request is required")
	}
	if c.Validation.Response.Required && strings.TrimSpace(c.Validation.Response.ErrorMessage) == "" {
		return errors.New("validation.response.error_message must not be empty when response is required")
	}
	if c.Validation.Request.Required && len(c.Validation.Request.RequiredHeaders) > 0 &&
		strings.TrimSpace(c.Validation.Request.RequiredHeaderErrorMessage) == "" {
		return errors.New("validation.request.required_header_error_message must not be empty when required_headers is set")
	}
	if c.Validation.Request.Required && len(c.Validation.Request.UniqueHeaders) > 0 &&
		strings.TrimSpace(c.Validation.Request.UniqueHeaderErrorMessage) == "" {
		return errors.New("validation.request.unique_header_error_message must not be empty when unique_headers is set")
	}
	for sheet, override := range c.SheetOverrides {
		if override.Validation == nil {
			continue
		}
		v := override.Validation
		if v.Request.Required && strings.TrimSpace(v.Request.ErrorMessage) == "" {
			return fmt.Errorf("sheet_overrides[%q].validation.request.error_message must not be empty when request is required", sheet)
		}
		if v.Response.Required && strings.TrimSpace(v.Response.ErrorMessage) == "" {
			return fmt.Errorf("sheet_overrides[%q].validation.response.error_message must not be empty when response is required", sheet)
		}
		if v.Request.Required && len(v.Request.RequiredHeaders) > 0 && strings.TrimSpace(v.Request.RequiredHeaderErrorMessage) == "" {
			return fmt.Errorf("sheet_overrides[%q].validation.request.required_header_error_message must not be empty when required_headers is set", sheet)
		}
		if v.Request.Required && len(v.Request.UniqueHeaders) > 0 && strings.TrimSpace(v.Request.UniqueHeaderErrorMessage) == "" {
			return fmt.Errorf("sheet_overrides[%q].validation.request.unique_header_error_message must not be empty when unique_headers is set", sheet)
		}
	}
	return nil
}
