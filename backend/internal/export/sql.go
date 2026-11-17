package export

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/database"
	"github.com/datadeck/datadeck/backend/internal/dbscript"
	"github.com/datadeck/datadeck/backend/internal/model"
)

// SQLOptions controls the SQL INSERT script export (PRF-02/T14, ADR-010 §14).
type SQLOptions struct {
	// Driver selects identifier quoting and literal rendering
	// (model.DriverPostgres / model.DriverMySQL / model.DriverSQLite).
	Driver string
	// Schema/Table identify the target relation (Schema is empty for engines
	// without a schema level).
	Schema, Table string
	// Transaction wraps the script in BEGIN;/COMMIT; so it either applies
	// completely or not at all.
	Transaction bool
	// CreateMode is "insert" (default), "create_if_not_exists", or "create".
	// The last two prepend a CREATE TABLE (from Structure) and append the
	// secondary indexes.
	CreateMode string
	// Structure is the table's metadata, required for a CREATE mode.
	Structure *model.TableStructure
}

// NewSQL returns a sink that writes one INSERT statement per row into w. All
// values are rendered as dialect-correct literals; strings are escaped, never
// concatenated as unquoted SQL text.
func NewSQL(w io.Writer, opts SQLOptions) database.ExportSink {
	return &sqlSink{writer: w, opts: opts}
}

type sqlSink struct {
	writer  io.Writer
	opts    SQLOptions
	quote   string
	table   string
	columns []string
	types   []string
}

func quoteFor(driver string) string {
	if driver == "mysql" {
		return "`"
	}
	return `"`
}

func (s *sqlSink) Begin(columns []model.QueryColumn) error {
	s.quote = quoteFor(s.opts.Driver)
	s.columns = make([]string, len(columns))
	s.types = make([]string, len(columns))
	for i, column := range columns {
		s.columns[i] = quoteIdentifier(s.quote, column.Name)
		s.types[i] = typeOfColumn(column)
	}
	s.table = quoteIdentifier(s.quote, s.opts.Table)
	if s.opts.Schema != "" {
		s.table = quoteIdentifier(s.quote, s.opts.Schema) + "." + s.table
	}
	if _, err := fmt.Fprintf(s.writer, "-- data for %s\n", s.table); err != nil {
		return err
	}
	if s.opts.Transaction {
		statement := "BEGIN;"
		if s.opts.Driver == "mysql" {
			statement = "START TRANSACTION;"
		}
		if _, err := fmt.Fprintln(s.writer, statement); err != nil {
			return err
		}
	}
	if err := s.writeCreate(); err != nil {
		return err
	}
	return nil
}

// writeCreate renders CREATE TABLE (from the structure) when a CREATE mode is
// requested. Constraints (PK/unique/check) are inline; the secondary indexes are
// written in End() after the data.
func (s *sqlSink) writeCreate() error {
	if s.opts.Structure == nil {
		return nil
	}
	if s.opts.CreateMode != "create" && s.opts.CreateMode != "create_if_not_exists" {
		return nil
	}
	ref := dbscript.TableRef{Schema: s.opts.Schema, Name: s.opts.Table}
	var rendered strings.Builder
	if err := dbscript.RenderCreateTable(&rendered, s.opts.Driver, ref, *s.opts.Structure); err != nil {
		return err
	}
	statement := strings.TrimSpace(rendered.String())
	if s.opts.CreateMode == "create_if_not_exists" {
		statement = strings.Replace(statement, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", 1)
	}
	if _, err := fmt.Fprintln(s.writer, statement); err != nil {
		return err
	}
	return nil
}

// writeIndexes appends the table's secondary indexes (for CREATE modes).
func (s *sqlSink) writeIndexes() error {
	if s.opts.Structure == nil {
		return nil
	}
	if s.opts.CreateMode != "create" && s.opts.CreateMode != "create_if_not_exists" {
		return nil
	}
	ref := dbscript.TableRef{Schema: s.opts.Schema, Name: s.opts.Table}
	return dbscript.RenderIndexes(s.writer, s.opts.Driver, ref, *s.opts.Structure)
}

func (s *sqlSink) Write(row []any) error {
	values := make([]string, len(row))
	for i, value := range row {
		values[i] = s.sqlLiteral(s.types[i], value)
	}
	_, err := fmt.Fprintf(s.writer, "INSERT INTO %s (%s) VALUES (%s);\n",
		s.table, strings.Join(s.columns, ", "), strings.Join(values, ", "))
	return err
}

func (s *sqlSink) End() error {
	if err := s.writeIndexes(); err != nil {
		return err
	}
	if s.opts.Transaction {
		_, err := fmt.Fprintln(s.writer, "COMMIT;")
		return err
	}
	return nil
}

// sqlLiteral renders one canonical value as a dialect-correct SQL literal.
func (s *sqlSink) sqlLiteral(typeName string, value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case json.RawMessage:
		// JSON arrives as an embedded document; quoting it yields a string that
		// the engine casts to the target column type on restore.
		return s.quoteString(string(v))
	case bool:
		if s.opts.Driver == "postgres" {
			if v {
				return "TRUE"
			}
			return "FALSE"
		}
		if v {
			return "1"
		}
		return "0"
	case []byte:
		return s.binaryLiteral(typeName, string(v))
	case string:
		return s.binaryLiteral(typeName, v)
	}
	// Non-string scalars (MySQL/SQLite numerics, dynamic SQLite values) render
	// as their quoted text so restore never depends on driver format quirks.
	text, _ := cellString(value)
	if isBlobType(typeName) {
		return hexLiteral(text)
	}
	return s.quoteString(text)
}

// binaryLiteral handles byte payloads: PostgreSQL bytea (canonical base64)
// becomes decode(...,'base64'); MySQL/SQLite blob types become hex literals;
// any other string returns with the driver's string escaping.
func (s *sqlSink) binaryLiteral(typeName, canonical string) string {
	if s.opts.Driver == "postgres" && typeName == "bytea" {
		// Base64 never contains a single quote.
		return "decode('" + canonical + "','base64')"
	}
	if isBlobType(typeName) {
		return hexLiteral(canonical)
	}
	return s.quoteString(canonical)
}

func isBlobType(typeName string) bool {
	name := strings.ToLower(typeName)
	return strings.Contains(name, "blob") || strings.Contains(name, "binary") ||
		strings.Contains(name, "bit") || strings.Contains(name, "geometry")
}

// hexLiteral converts a canonical base64 value into an X'…' binary literal.
func hexLiteral(base64Text string) string {
	decoded, err := base64.StdEncoding.DecodeString(base64Text)
	if err != nil {
		decoded = []byte(base64Text)
	}
	return "X'" + hex.EncodeToString(decoded) + "'"
}

// quoteString escapes a string for the driver. All drivers double single
// quotes; MySQL also escapes backslashes (its default backslash-escape mode).
func (s *sqlSink) quoteString(text string) string {
	var b strings.Builder
	b.Grow(len(text) + 2)
	b.WriteByte('\'')
	for i := 0; i < len(text); i++ {
		switch c := text[i]; {
		case c == '\'':
			b.WriteString("''")
		case c == '\\' && s.opts.Driver == "mysql":
			b.WriteString("\\\\")
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}
