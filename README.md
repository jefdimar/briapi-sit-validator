# BRIAPI SIT Validator

A Go HTTP service that validates Excel files submitted by BRIAPI partners as part of the System Integration Testing (SIT) process.

## Requirements

- Go 1.22+
- golangci-lint (for `make lint`)
- Docker (for `make docker`)

## Quick Start

```bash
# Run the server (default port 8080) — vendor directory is committed, no download needed
go run -mod=vendor ./cmd/server

# Or build a binary first
go build -mod=vendor -o bin/sit-validator ./cmd/server
./bin/sit-validator

# Validate a file (JSON response)
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

## Postman Collection

A ready-to-use Postman collection and environment are included in the `postman/` directory.

### Files

| File | Description |
|---|---|
| `postman/BRIAPI_SIT_Validator.postman_collection.json` | Collection with all requests and automated test scripts |
| `postman/BRIAPI_SIT_Validator.postman_environment.json` | Environment variables (base URL, file path, etc.) |

### Import Steps

1. Open Postman.
2. Click **Import** → drag-and-drop (or select) both files from the `postman/` directory.
3. Select the **BRIAPI SIT Validator — Local** environment from the environment dropdown (top-right).
4. Set the `sit_file_path` variable to the absolute path of your SIT Excel file:
   - Go to **Environments** → **BRIAPI SIT Validator — Local**
   - Set `sit_file_path` current value, e.g. `/Users/you/Downloads/_SIT__09_Lampiran.xlsx`
5. Start the server locally (`go run -mod=vendor ./cmd/server`).
6. Run individual requests or use **Run Collection** to execute all requests in sequence.

### Collection Structure

| Folder | Requests |
|---|---|
| 01 - Health | `GET /api/v1/health` — verify the service is up |
| 02 - Validate (JSON) | Validate all sheets · Validate with sheet filter |
| 03 - Validate (Excel) | Download annotated Excel report · Download with sheet filter |
| 04 - Error Cases | Missing file · Invalid file type · File too large (synthetic) |

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `base_url` | `http://localhost:8080` | Server address. Change for remote deployments. |
| `api_version` | `v1` | API version prefix. |
| `sit_file_path` | *(empty)* | **Must be set.** Absolute path to your SIT `.xlsx` file. |
| `sheet_filter` | *(empty)* | Optional comma-separated sheet names. Leave empty to validate all sheets. |

### Running with Newman (CLI)

```bash
# Install Newman
npm install -g newman

# Run the full collection
newman run postman/BRIAPI_SIT_Validator.postman_collection.json \
  --environment postman/BRIAPI_SIT_Validator.postman_environment.json \
  --env-var "sit_file_path=/absolute/path/to/your_sit_file.xlsx"
```

## Development

```bash
go test -mod=vendor ./...          # run tests
go build -mod=vendor -o bin/sit-validator ./cmd/server  # build binary

# If make is available:
make test    # run tests
make lint    # run golangci-lint
make build   # build binary to bin/sit-validator
make docker  # build Docker image
```
