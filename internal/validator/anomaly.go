package validator

import (
	"fmt"
	"strings"

	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/model"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
)

// detectSheetAnomalies checks the structural integrity of a sheet against the
// expected layout defined in cfg. It returns a list of human-readable anomaly
// strings. Any anomaly means the sheet does not conform to the expected structure
// and should be treated as validation-incomplete regardless of cell content.
//
// Checks performed:
//   - Header row (cfg.Excel.HeaderRow) exists within the sheet
//   - Data start row (cfg.Excel.DataStartRow) exists within the sheet
//   - At least one test case row has a non-empty "No" column value
//   - Every configured metadata field is present (not "missing")
func detectSheetAnomalies(p *parser.File, sheet string, cfg *config.Config, meta map[string]model.MetaField) []string {
	var anomalies []string

	rows, err := p.GetRows(sheet)
	if err != nil {
		return []string{fmt.Sprintf("Gagal membaca sheet: %s", err.Error())}
	}

	// Check that the header row exists.
	headerRowIdx := cfg.Excel.HeaderRow - 1 // convert to 0-indexed
	if headerRowIdx >= len(rows) {
		anomalies = append(anomalies,
			fmt.Sprintf("Header row %d tidak ditemukan (sheet hanya memiliki %d baris)",
				cfg.Excel.HeaderRow, len(rows)))
		// No point checking data rows if the header is missing.
		return anomalies
	}

	// Check that the data start row exists.
	dataStartIdx := cfg.Excel.DataStartRow - 1 // convert to 0-indexed
	if dataStartIdx >= len(rows) {
		anomalies = append(anomalies,
			fmt.Sprintf("Baris data dimulai di row %d tidak ditemukan (sheet hanya memiliki %d baris)",
				cfg.Excel.DataStartRow, len(rows)))
		return anomalies
	}

	// Check that at least one test case exists (non-empty "No" column).
	hasData := false
	for i := dataStartIdx; i < len(rows); i++ {
		if cfg.Excel.Columns.No < len(rows[i]) && strings.TrimSpace(rows[i][cfg.Excel.Columns.No]) != "" {
			hasData = true
			break
		}
	}
	if !hasData {
		anomalies = append(anomalies,
			"Sheet tidak memiliki test case (kolom No kosong di semua baris data)")
	}

	// Elevate missing metadata fields as structural anomalies.
	// Iterate in a deterministic order using the config keys.
	for _, m := range cfg.Excel.Metadata {
		if field, ok := meta[m.Key]; ok && field.Status == "missing" {
			anomalies = append(anomalies,
				fmt.Sprintf("Metadata '%s' tidak diisi", m.Key))
		}
	}

	return anomalies
}
