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
	Server     ServerConfig     `yaml:"server"`
	Excel      ExcelConfig      `yaml:"excel"`
	Validation ValidationConfig `yaml:"validation"`
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
	Request  FieldValidation  `yaml:"request"`
	Response FieldValidation  `yaml:"response"`
	Result   ResultValidation `yaml:"result"`
	Notes    NotesValidation  `yaml:"notes"`
}

type FieldValidation struct {
	Required             bool     `yaml:"required"`
	EmptySentinelValues  []string `yaml:"empty_sentinel_values"`
	ErrorMessage         string   `yaml:"error_message"`
}

type ResultValidation struct {
	Required            bool     `yaml:"required"`
	AllowedValues       []string `yaml:"allowed_values"`
	ErrorMessage        string   `yaml:"error_message"`         // used when result is empty
	InvalidValueMessage string   `yaml:"invalid_value_message"` // used when result is not in allowed_values
}

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
	if len(c.Validation.Result.AllowedValues) == 0 {
		return errors.New("validation.result.allowed_values must not be empty")
	}
	msgs := map[string]string{
		"validation.request.error_message":              c.Validation.Request.ErrorMessage,
		"validation.response.error_message":             c.Validation.Response.ErrorMessage,
		"validation.result.error_message":               c.Validation.Result.ErrorMessage,
		"validation.result.invalid_value_message":       c.Validation.Result.InvalidValueMessage,
		"validation.notes.error_message":                c.Validation.Notes.ErrorMessage,
	}
	for field, msg := range msgs {
		if strings.TrimSpace(msg) == "" {
			return fmt.Errorf("%s must not be empty", field)
		}
	}
	return nil
}
