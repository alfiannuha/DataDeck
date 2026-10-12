package export

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// ErrRowLimit is returned by the XLSX sink when the sheet would exceed
// excelize.MaxRows rows (1,048,576) — the workbook format's hard ceiling. The
// handler maps it to EXPORT_LIMIT_EXCEEDED.
var ErrRowLimit = fmt.Errorf("export: row limit exceeded (%d rows, header included)", excelize.TotalRows)

// NewXLSX returns a sink streaming the export into a single-sheet .xlsx
// workbook. Cells are written as inline strings (the StreamWriter's setInlineStr
// path — never a formula), bools as booleans, numeric values as numbers, and
// NULL as a blank cell. BIGINT and DECIMAL values arrive as strings from the
// canonical encoders and land in plain text cells, preserving exact precision.
func NewXLSX(w io.Writer) database.ExportSink {
	file := excelize.NewFile()
	// The default workbook ships with "Sheet1"; NewStreamWriter cannot fail for
	// a sheet that exists.
	sw, _ := file.NewStreamWriter("Sheet1")
	return &xlsxSink{file: file, output: w, writer: sw}
}

type xlsxSink struct {
	file   *excelize.File
	output io.Writer
	writer *excelize.StreamWriter
	row    int
}

func (s *xlsxSink) Begin(columns []model.QueryColumn) error {
	headers := make([]any, len(columns))
	for i, column := range columns {
		headers[i] = column.Name
	}
	return s.writeRow(headers)
}

func (s *xlsxSink) Write(row []any) error {
	return s.writeRow(row)
}

func (s *xlsxSink) writeRow(values []any) error {
	if s.row >= excelize.TotalRows {
		return fmt.Errorf("%w", ErrRowLimit)
	}
	cell, err := excelize.CoordinatesToCellName(1, s.row+1)
	if err != nil {
		return err
	}
	cellValues := make([]any, len(values))
	for i, value := range values {
		cellValues[i] = xlsxCell(value)
	}
	if err := s.writer.SetRow(cell, cellValues); err != nil {
		return err
	}
	s.row++
	return nil
}

func (s *xlsxSink) End() error {
	if err := s.writer.Flush(); err != nil {
		return err
	}
	_, err := s.file.WriteTo(s.output)
	return err
}

// xlsxCell maps a canonical model value to a spreadsheet cell value. Strings
// (including BIGINT/DECIMAL precision text) stay inline string cells; NULL
// becomes a blank cell.
func xlsxCell(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case bool:
		return v
	case float64:
		return v
	case float32:
		return float64(v)
	case json.RawMessage:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
