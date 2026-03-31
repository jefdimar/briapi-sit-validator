package parser

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// newXlsxBytes creates a minimal in-memory xlsx workbook and returns its bytes.
func newXlsxBytes(t *testing.T, setup func(f *excelize.File)) []byte {
	t.Helper()
	f := excelize.NewFile()
	setup(f)
	var buf bytes.Buffer
	_, err := f.WriteTo(&buf)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return buf.Bytes()
}

func TestFromReader_ValidXlsx(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "hello")
	})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)
	require.NotNil(t, p)
	defer p.Close()
}

func TestFromReader_InvalidData(t *testing.T) {
	_, err := FromReader(strings.NewReader("not an xlsx file"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse excel")
}

func TestSheetNames(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {
		f.NewSheet("ProductA")
		f.NewSheet("ProductB")
	})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer p.Close()

	names := p.SheetNames()
	assert.Contains(t, names, "Sheet1")
	assert.Contains(t, names, "ProductA")
	assert.Contains(t, names, "ProductB")
}

func TestGetCellValue_ExistingCell(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "B3", "PT Bank XYZ")
	})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer p.Close()

	// row=3, col=2 → B3
	val, err := p.GetCellValue("Sheet1", 3, 2)
	require.NoError(t, err)
	assert.Equal(t, "PT Bank XYZ", val)
}

func TestGetCellValue_EmptyCell(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer p.Close()

	val, err := p.GetCellValue("Sheet1", 1, 1)
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestGetCellValue_InvalidCoordinates(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer p.Close()

	// row=0 is invalid (must be >= 1)
	_, err = p.GetCellValue("Sheet1", 0, 0)
	assert.Error(t, err)
}

func TestGetRows(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "no")
		f.SetCellValue("Sheet1", "B1", "service")
		f.SetCellValue("Sheet1", "A2", "8.1")
		f.SetCellValue("Sheet1", "B2", "Any Service")
	})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer p.Close()

	rows, err := p.GetRows("Sheet1")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, []string{"no", "service"}, rows[0])
	assert.Equal(t, []string{"8.1", "Any Service"}, rows[1])
}

func TestGetRows_UnknownSheet(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer p.Close()

	_, err = p.GetRows("NoSuchSheet")
	assert.Error(t, err)
}

func TestRaw_ReturnsUnderlying(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {
		f.SetCellValue("Sheet1", "A1", "x")
	})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer p.Close()

	assert.NotNil(t, p.Raw())
}

func TestClose(t *testing.T) {
	data := newXlsxBytes(t, func(f *excelize.File) {})

	p, err := FromReader(bytes.NewReader(data))
	require.NoError(t, err)

	err = p.Close()
	assert.NoError(t, err)
}
