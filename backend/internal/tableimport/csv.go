// Package tableimport parses uploaded table data into cells for PRF-02/T15
// table import. It is intentionally smaller than the export package: it only
// understands CSV (RFC 4180) and restricted, INSERT-only SQL scripts — the two
// formats T15 supports (XLSX import is future work, ADR-010 §15). It depends
// only on the model package (never database), matching the export package's
// structural boundary, so the database layer stays free of encoding concerns.
//
// Output cells carry an explicit NULL flag so callers can distinguish an empty
// CSV field from the empty string, exactly as ADR-010 §13 requires.
package tableimport

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// Sentinel errors. Callers match them with errors.Is.
var (
	// ErrTooManyColumns marks a record wider than MaxColumns.
	ErrTooManyColumns = errors.New("tableimport: record has too many columns")
	// ErrRowTooLarge marks a single record exceeding MaxRowBytes.
	ErrRowTooLarge = errors.New("tableimport: record exceeds the size limit")
	// ErrUnterminatedQuote marks a record with an unclosed quoted field.
	ErrUnterminatedQuote = errors.New("tableimport: unterminated quoted field")
)

// CSVOptions controls CSV parsing.
type CSVOptions struct {
	// Delimiter defaults to ',' (RFC 4180 compatible). Tab and semicolon are
	// the common alternatives.
	Delimiter rune
	// HeaderRow treats the first record as a header (returned as columns,
	// never emitted as data).
	HeaderRow bool
	// MaxColumns bounds the width of every record (formula bombs / DOS guard).
	MaxColumns int
	// MaxRowBytes bounds the encoding size of a single record.
	MaxRowBytes int
}

// csvLimits applies the CSVOptions defaults.
func (o CSVOptions) limits() (rune, int, int) {
	delim := o.Delimiter
	if delim == 0 {
		delim = ','
	}
	cols := o.MaxColumns
	if cols <= 0 {
		cols = 1000
	}
	bytes := o.MaxRowBytes
	if bytes <= 0 {
		bytes = 1 << 20 // 1 MiB
	}
	return delim, cols, bytes
}

// ParseCSV streams a CSV document. When HeaderRow is set the first record is
// returned as columns and excluded from emit; data records are passed to emit
// in order. The returned rowCount counts only emitted (data) records.
//
// cellString is called for every emitted value, so callers that need the raw
// cell only pay for conversion once. ParseCSV stops at the first error: an
// emit error is returned unchanged (and cancels parsing).
func ParseCSV(r io.Reader, opts CSVOptions, emit func(cell []model.ImportCell) error) (columns []string, rowCount int, err error) {
	delim, cols, rowBytes := opts.limits()
	cr := &csvRowReader{br: bufio.NewReaderSize(r, 64<<10), delim: byte(delim), quote: '"', maxCols: cols, maxRowBytes: rowBytes}
	first := true
	for {
		row, err := cr.readRow()
		if errors.Is(err, io.EOF) {
			return columns, rowCount, nil
		}
		if err != nil {
			return columns, rowCount, err
		}
		if first {
			first = false
			if opts.HeaderRow {
				columns = make([]string, len(row))
				for i, cell := range row {
					columns[i] = unguardFormula(cell.Value, cell.IsNull)
				}
				continue
			}
			columns = make([]string, len(row))
			for i := range columns {
				columns[i] = fmt.Sprintf("column_%d", i+1)
			}
		} else {
			rowCount++
		}
		if len(row) != len(columns) {
			if opts.HeaderRow {
				return columns, rowCount, fmt.Errorf("row %d: expected %d columns, got %d", rowCount+1, len(columns), len(row))
			}
			return columns, rowCount, fmt.Errorf("row %d: expected %d columns, got %d", rowCount, len(columns), len(row))
		}
		for i := range row {
			row[i].Value = unguardFormula(row[i].Value, row[i].IsNull)
		}
		if err := emit(row); err != nil {
			return columns, rowCount, err
		}
	}
}

// csvRowReader is a minimal streaming RFC 4180 reader that preserves quotedness,
// which encoding/csv cannot do (it collapses "" to an empty string). It reads
// one record at a time, is fully streaming, enforces width/size caps, and
// tolerates a leading UTF-8 BOM. Accepted line endings: CRLF, LF, CR.
type csvRowReader struct {
	br          *bufio.Reader
	delim       byte
	quote       byte
	maxCols     int
	maxRowBytes int
	started     bool
	recordNo    int
}

func (c *csvRowReader) readRow() ([]model.ImportCell, error) {
	// Skip a leading UTF-8 BOM on the first read.
	if !c.started {
		c.started = true
		if b, err := c.br.Peek(3); err == nil {
			if len(b) == 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
				_, _ = c.br.Discard(3)
			}
		}
	}

	c.recordNo++
	var row []model.ImportCell
	var field strings.Builder
	quoted := false
	inQuoted := false
	startOfField := true
	fieldNo := 0
	rowBytes := 0

	for {
		b, err := c.br.ReadByte()
		if errors.Is(err, io.EOF) {
			// A trailing unterminated quoted field is malformed.
			if inQuoted {
				return nil, fmt.Errorf("row %d: %w", c.recordNo, ErrUnterminatedQuote)
			}
			if startOfField {
				if row == nil && field.Len() == 0 {
					// EOF without any record.
					return nil, io.EOF
				}
				// We hit EOF exactly at a record boundary; a final empty field is
				// only a real field if it followed a delimiter.
				if row != nil {
					row = append(row, model.ImportCell{IsNull: true})
				}
			} else {
				row = append(row, model.ImportCell{IsNull: !quoted && field.Len() == 0, Value: field.String()})
			}
			return row, nil
		}
		if err != nil {
			return nil, err
		}

		switch {
		case startOfField && b == c.quote:
			// Quoted field: read until the closing quote; "" is an escaped quote.
			quoted = true
			startOfField = false
			inQuoted = true
			for {
				q, err := c.br.ReadByte()
				if errors.Is(err, io.EOF) {
					return nil, fmt.Errorf("row %d: %w", c.recordNo, ErrUnterminatedQuote)
				}
				if err != nil {
					return nil, err
				}
				if q == c.quote {
					next, err := c.br.Peek(1)
					if err == nil && next[0] == c.quote {
						_, _ = c.br.Discard(1)
						field.WriteByte(c.quote)
						continue
					}
					inQuoted = false
					// A quote is only valid immediately before a delimiter or EOL.
					if err == nil && next[0] != c.delim && next[0] != '\r' && next[0] != '\n' {
						return nil, fmt.Errorf("row %d: unexpected %q after closing quote", c.recordNo, next[0])
					}
					break
				}
				if rowBytes+1 > c.maxRowBytes {
					return nil, fmt.Errorf("row %d: %w", c.recordNo, ErrRowTooLarge)
				}
				rowBytes++
				field.WriteByte(q)
			}
		case b == c.delim:
			if startOfField {
				// An empty field between delimiters (",,") is NULL.
				row = append(row, model.ImportCell{IsNull: true})
			} else {
				row = append(row, model.ImportCell{IsNull: !quoted && field.Len() == 0, Value: field.String()})
			}
			field.Reset()
			quoted = false
			startOfField = true
			fieldNo++
			if fieldNo+1 > c.maxCols {
				return nil, fmt.Errorf("row %d: %w", c.recordNo, ErrTooManyColumns)
			}
		case b == '\r' || b == '\n':
			// End of record (CRLF, CR, or LF).
			if inQuoted {
				field.WriteByte(b)
				continue
			}
			if b == '\r' {
				if n, err := c.br.Peek(1); err == nil && n[0] == '\n' {
					_, _ = c.br.Discard(1)
				}
			}
			if row == nil && startOfField {
				// A fully blank line carries no record; skip it.
				continue
			}
			if !startOfField {
				row = append(row, model.ImportCell{IsNull: !quoted && field.Len() == 0, Value: field.String()})
			} else if row != nil {
				// The record ended right after a delimiter: the final field is
				// an unquoted empty string, i.e. NULL.
				row = append(row, model.ImportCell{IsNull: true})
			}
			return row, nil
		default:
			if rowBytes+1 > c.maxRowBytes {
				return nil, fmt.Errorf("row %d: %w", c.recordNo, ErrRowTooLarge)
			}
			rowBytes++
			startOfField = false
			field.WriteByte(b)
		}
	}
}

// unguardFormula inverts the export package's spreadsheet-formula guard: a
// leading "'" that DataDeck added in front of a formula-looking value (starts
// with = + @ -, or - followed by a non-digit) is stripped so export→import
// round-trips. Values that legitimately start with "'" not followed by a
// formula marker, and plain numbers, are left untouched.
func unguardFormula(value string, isNull bool) string {
	if isNull || len(value) < 2 || value[0] != '\'' {
		return value
	}
	rest := value[1:]
	if isFormulaMarker(rest) {
		return rest
	}
	return value
}

func isFormulaMarker(value string) bool {
	if value == "" {
		return false
	}
	switch value[0] {
	case '=', '+', '@':
		return true
	case '-':
		// A plain negative number (= + optional digit after the minus) is not a
		// formula, mirrors export.guardFormula.
		return !nextIsDigit(value[1:])
	default:
		return false
	}
}

func nextIsDigit(s string) bool {
	return s != "" && s[0] >= '0' && s[0] <= '9'
}
