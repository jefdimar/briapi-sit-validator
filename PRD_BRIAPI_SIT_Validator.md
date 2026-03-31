# PRD: BRIAPI SIT Excel Validator Service

## Overview

A Go HTTP service that validates Excel files submitted by BRIAPI partners as part of the
System Integration Testing (SIT) process. The file (`_SIT__09_Lampiran_Skenario_Functional_Test.xlsx`)
contains 23 product sheets, each with test case scenarios that partners must fill in
(Request, Response, Result, Notes). This service checks for missing or unfilled fields
and returns a structured validation report without executing any HTTP calls.

---

## Goals

- Parse partner-submitted Excel SIT files
- Detect unfilled or template-default values per test case row
- Validate header metadata (provider name, partner name, test date)
- Return a structured JSON report per sheet and overall summary
- Optionally export the validation report back as an annotated Excel file
- Be configurable via YAML so new sheets or rule changes don't require code changes

---

## Tech Stack

| Layer | Library |
|---|---|
| Language | Go 1.22+ |
| HTTP Framework | Gin (`github.com/gin-gonic/gin`) |
| Excel Parser | excelize v2 (`github.com/xuri/excelize/v2`) |
| Config | YAML (`gopkg.in/yaml.v3`) |
| Logging | `log/slog` (stdlib) |
| Testing | `testing` + `testify` |

---

## Project Structure

```
briapi-sit-validator/
├── cmd/
│   └── server/
│       └── main.go                  # entrypoint, loads config, starts Gin
├── internal/
│   ├── config/
│   │   └── config.go                # load and parse rules.yaml
│   ├── parser/
│   │   └── excel.go                 # open xlsx, iterate sheets and rows
│   ├── validator/
│   │   ├── validator.go             # orchestrates validation per sheet
│   │   ├── metadata.go              # validate header metadata rows
│   │   └── testcase.go              # validate individual test case rows
│   ├── reporter/
│   │   ├── json.go                  # build JSON report
│   │   └── excel.go                 # write annotated Excel report (optional)
│   └── model/
│       └── model.go                 # all shared structs
├── config/
│   └── rules.yaml                   # validation rules (sheet layout, empty sentinel values)
├── testdata/
│   └── sample_sit.xlsx              # sample/fixture for unit tests
├── .github/
│   └── workflows/
│       └── ci.yml                   # lint + test on push
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## API Endpoints

### `POST /api/v1/validate`

Upload the SIT Excel file for validation.

**Request**
```
Content-Type: multipart/form-data
Field: file  (xlsx binary)
```

**Query Params (optional)**

| Param | Type | Default | Description |
|---|---|---|---|
| `format` | string | `json` | `json` or `excel` — response format |
| `sheets` | string | all | Comma-separated sheet names to validate (e.g. `Interbank Transfer,QR MPM`) |

**Response 200 — JSON**
```json
{
  "status": "incomplete",
  "summary": {
    "total_sheets": 23,
    "sheets_ok": 18,
    "sheets_incomplete": 5,
    "total_test_cases": 187,
    "test_cases_ok": 160,
    "test_cases_incomplete": 27
  },
  "sheets": [
    {
      "sheet_name": "Interbank Transfer",
      "metadata": {
        "provider_name": { "value": "PT Bank XYZ", "status": "ok" },
        "partner_name":  { "value": "",             "status": "missing" },
        "test_date":     { "value": "2025-03-01",   "status": "ok" }
      },
      "summary": {
        "total": 13,
        "ok": 10,
        "incomplete": 3
      },
      "test_cases": [
        {
          "no": "8.1",
          "service": "Any Service",
          "scenario": "Access Token Invalid",
          "status": "incomplete",
          "issues": [
            "Request belum diisi (harus berupa curl)",
            "Response Body belum diisi"
          ]
        },
        {
          "no": "8.2",
          "service": "Any Service",
          "scenario": "Unauthorized . Signature",
          "status": "ok",
          "issues": []
        }
      ]
    }
  ]
}
```

**Response 200 — Excel**

Returns annotated `.xlsx` file with an added `Validation Result` column per sheet,
cells highlighted red for issues and green for OK rows.

```
Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
Content-Disposition: attachment; filename="sit_validation_report.xlsx"
```

**Response 400**
```json
{ "error": "invalid file format: expected .xlsx" }
```

**Response 422**
```json
{ "error": "file has no recognizable product sheets" }
```

---

### `GET /api/v1/health`

```json
{ "status": "ok", "version": "1.0.0" }
```

---

## Data Models (`internal/model/model.go`)

```go
type ValidationReport struct {
    Status  string         `json:"status"`  // "ok" | "incomplete"
    Summary GlobalSummary  `json:"summary"`
    Sheets  []SheetReport  `json:"sheets"`
}

type GlobalSummary struct {
    TotalSheets          int `json:"total_sheets"`
    SheetsOK             int `json:"sheets_ok"`
    SheetsIncomplete     int `json:"sheets_incomplete"`
    TotalTestCases       int `json:"total_test_cases"`
    TestCasesOK          int `json:"test_cases_ok"`
    TestCasesIncomplete  int `json:"test_cases_incomplete"`
}

type SheetReport struct {
    SheetName string              `json:"sheet_name"`
    Metadata  map[string]MetaField `json:"metadata"`
    Summary   SheetSummary        `json:"summary"`
    TestCases []TestCaseResult    `json:"test_cases"`
}

type MetaField struct {
    Value  string `json:"value"`
    Status string `json:"status"` // "ok" | "missing"
}

type SheetSummary struct {
    Total      int `json:"total"`
    OK         int `json:"ok"`
    Incomplete int `json:"incomplete"`
}

type TestCaseResult struct {
    No       string   `json:"no"`
    Service  string   `json:"service"`
    Scenario string   `json:"scenario"`
    Status   string   `json:"status"` // "ok" | "incomplete"
    Issues   []string `json:"issues"`
}
```

---

## Config (`config/rules.yaml`)

```yaml
server:
  port: 8080
  max_upload_size_mb: 20

excel:
  skip_sheets:
    - Changelog
    - Daftar Isi
    - Step
  header_row: 9          # 1-indexed row where column headers are
  data_start_row: 10     # first test case row

  metadata:
    - key: provider_name
      label: "Nama Penyedia Layanan:"
      row: 3
      col: 1             # column B (0-indexed: A=0, B=1)
    - key: partner_name
      label: "Nama Pengguna Layanan :"
      row: 5
      col: 1
    - key: test_date
      label: "Tanggal Pengujian:"
      row: 6
      col: 1

  columns:
    no:              0   # Column A
    service:         1   # Column B
    scenario:        2   # Column C
    expected_result: 3   # Column D
    request:         4   # Column E
    response:        5   # Column F
    result:          6   # Column G
    notes:           7   # Column H

validation:
  request:
    required: true
    empty_sentinel_values:
      - "URL Endpoint:\n Header Request:\n Request Body:"
      - "URL Endpoint: Header Request: Request Body:"
      - ""
    error_message: "Request belum diisi (harus berupa curl: URL Endpoint, Header Request, Request Body)"

  response:
    required: true
    empty_sentinel_values:
      - "Response Body:"
      - ""
    error_message: "Response Body belum diisi"

  result:
    required: true
    allowed_values:
      - "Passed"
      - "Not Passed"
      - "passed"
      - "not passed"
      - "PASSED"
      - "NOT PASSED"
    error_message: "Result belum diisi (isi dengan: Passed / Not Passed)"

  notes:
    required_if_result: "not passed"   # case-insensitive match
    error_message: "Notes wajib diisi jika Result adalah Not Passed"
```

---

## Validation Logic

### Metadata Validation (`internal/validator/metadata.go`)

For each product sheet:
1. Read cell at `(metadata.row, metadata.col + 1)` — the cell to the right of the label
2. Trim whitespace
3. If empty → `MetaField{Value: "", Status: "missing"}`
4. Else → `MetaField{Value: val, Status: "ok"}`

### Test Case Validation (`internal/validator/testcase.go`)

For each row from `data_start_row` onwards (stop when `col[no]` is nil/empty):

```
issues = []

1. Check Request column
   - trim whitespace
   - if empty OR value matches any sentinel → append error_message

2. Check Response column
   - trim whitespace
   - if empty OR value matches any sentinel → append error_message

3. Check Result column
   - trim whitespace, lowercase
   - if empty → append error_message
   - if not in allowed_values → append "Result tidak valid, harus Passed atau Not Passed"

4. Check Notes column (conditional)
   - if Result (lowercase) contains "not passed"
     AND Notes is empty → append error_message

status = "ok" if len(issues) == 0 else "incomplete"
```

---

## Excel Report Output (`internal/reporter/excel.go`)

When `format=excel`:
1. Copy the original workbook
2. For each product sheet, append column `I` with header `Validation Result`
3. Per row: write `✓ OK` (green fill) or issue list joined by ` | ` (red fill)
4. For metadata section: add a comment or highlight the cell if missing
5. Stream as download

---

## Error Handling

| Scenario | HTTP Status | Response |
|---|---|---|
| No file in multipart | 400 | `{ "error": "file is required" }` |
| Non-xlsx file | 400 | `{ "error": "invalid file format: expected .xlsx" }` |
| File exceeds max size | 413 | `{ "error": "file too large, max 20MB" }` |
| xlsx parse failure | 422 | `{ "error": "cannot parse excel file: <reason>" }` |
| No product sheets found | 422 | `{ "error": "no recognizable product sheets found" }` |
| Internal error | 500 | `{ "error": "internal server error" }` |

---

## Non-Functional Requirements

- **No database** — stateless, in-memory processing per request
- **No HTTP calls** — pure file parsing only
- **Concurrency** — each sheet parsed concurrently via goroutines + `sync.WaitGroup`
- **Max file size** — configurable via `rules.yaml`, default 20MB
- **Response time** — < 3s for a 23-sheet file
- **Graceful shutdown** — handle `SIGTERM` / `SIGINT`
- **Structured logging** — `slog` with `request_id`, sheet name, row number context

---

## Makefile Targets

```makefile
run       # go run ./cmd/server
build     # go build -o bin/sit-validator ./cmd/server
test      # go test ./...
lint      # golangci-lint run
docker    # docker build -t briapi-sit-validator .
```

---

## Git Conventions

- No Claude footer or co-author in commits
- Commit message format: `<type>(<scope>): <message>`
  - e.g. `feat(validator): add notes conditional check`
  - e.g. `fix(parser): handle nil row in read-only mode`
- Types: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`

---

## Out of Scope (v1)

- Executing actual HTTP requests against partner endpoints
- Authentication / API key for the service itself
- Persistent storage of validation results
- UI / frontend
- Support for `.xls` (legacy) format
