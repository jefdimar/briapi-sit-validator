# BRIAPI SIT Validator

A Go HTTP service that validates Excel files submitted by BRIAPI partners as part of the System Integration Testing (SIT) process.

## Requirements

- Go 1.22+
- golangci-lint (for `make lint`)
- Docker (for `make docker`)

## Quick Start

```bash
# Install dependencies
go mod download

# Run the server (default port 8080)
make run

# Validate a file
curl -F "file=@your_sit_file.xlsx" http://localhost:8080/api/v1/validate

# Get annotated Excel report
curl -F "file=@your_sit_file.xlsx" \
  "http://localhost:8080/api/v1/validate?format=excel" \
  -o sit_validation_report.xlsx
```

## API

### `POST /api/v1/validate`

| Parameter | Type | Description |
|---|---|---|
| `file` | multipart/form-data | Required. `.xlsx` file to validate |
| `format` | query string | `json` (default) or `excel` |
| `sheets` | query string | Comma-separated sheet names to validate |

### `GET /api/v1/health`

Returns service health and version.

## Configuration

Edit `config/rules.yaml` to adjust:
- Server port and max upload size
- Sheets to skip (e.g. Changelog, Daftar Isi)
- Metadata row/column positions
- Column index mapping
- Validation rules and sentinel values

## Development

```bash
make test    # run tests
make lint    # run golangci-lint
make build   # build binary to bin/sit-validator
make docker  # build Docker image
```
