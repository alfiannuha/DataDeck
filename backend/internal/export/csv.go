package export

import (
	"encoding/csv"
	"io"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// NewCSV returns a sink streaming RFC 4180 (CRLF-terminated) CSV, optionally
// with a header row. Data cells are guarded against spreadsheet formulas; the
// header row is guarded identically because a sheet header beginning with
// = + - @ is an equally valid injection vector.
func NewCSV(w io.Writer, includeHeaders bool) database.ExportSink {
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	return &csvSink{writer: cw, includeHeaders: includeHeaders}
}

type csvSink struct {
	writer         *csv.Writer
	includeHeaders bool
	begun          bool
}

func (s *csvSink) Begin(columns []model.QueryColumn) error {
	s.begun = true
	if !s.includeHeaders {
		return nil
	}
	row := make([]string, len(columns))
	for i, column := range columns {
		row[i] = guardFormula(column.Name)
	}
	return s.writer.Write(row)
}

func (s *csvSink) Write(row []any) error {
	if !s.begun {
		return nil
	}
	out := make([]string, len(row))
	for i, value := range row {
		text, _ := cellString(value) // NULL → empty field (ADR-010 §13)
		out[i] = guardFormula(text)
	}
	return s.writer.Write(out)
}

func (s *csvSink) End() error {
	s.writer.Flush()
	return s.writer.Error()
}
