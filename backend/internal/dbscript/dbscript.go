// Package dbscript implements database-level SQL dump export and the restricted,
// DataDeck-format restore validation for PRF-02/T17.
//
// It is deliberately self-contained: no external tooling (pg_dump/mysqldump/
// sqlite3) is invoked, so the single-binary, CGO-free distribution is preserved.
// Export renders a dialect-correct SQL script for the base tables of one
// database; restore only accepts artifacts produced by this exporter, validates
// them against a statement allowlist, and refuses to touch a non-empty database.
package dbscript

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// FormatVersion is the artifact format version this build writes and the only
// version restore accepts.
const FormatVersion = 1

// ErrNotDataDeckArtifact marks input that is not a DataDeck SQL export.
var ErrNotDataDeckArtifact = errors.New("dbscript: not a DataDeck export artifact")

// Mode selects what an export contains. Only supported modes are offered.
type Mode string

const (
	ModeSchemaData Mode = "schema_data"
	ModeSchema     Mode = "schema"
	ModeData       Mode = "data"
)

// SupportedModes lists the export modes this build implements.
var SupportedModes = []Mode{ModeSchemaData, ModeSchema, ModeData}

// ParseMode validates and normalizes an export mode string.
func ParseMode(raw string) (Mode, error) {
	switch Mode(strings.TrimSpace(strings.ToLower(raw))) {
	case "", ModeSchemaData:
		return ModeSchemaData, nil
	case ModeSchema:
		return ModeSchema, nil
	case ModeData:
		return ModeData, nil
	default:
		return "", fmt.Errorf("unsupported export mode %q", raw)
	}
}

// Manifest is the machine-readable header that makes an artifact identifiable
// and bindable to an explicit engine + database. It is written as leading SQL
// comment lines so the artifact stays a valid SQL script.
type Manifest struct {
	FormatVersion int
	Engine        string
	Database      string
	Mode          Mode
	GeneratedAt   time.Time
	Tables        int
	Indexes       int
	ForeignKeys   int
	// Excluded lists supported-scope objects that were deliberately not
	// exported (e.g. views), so the artifact never claims full coverage.
	Excluded []string
}

// WriteHeader writes the manifest as leading comment lines.
func WriteHeader(w io.Writer, m Manifest) error {
	if m.FormatVersion == 0 {
		m.FormatVersion = FormatVersion
	}
	excluded := ""
	if len(m.Excluded) > 0 {
		excluded = strings.Join(m.Excluded, ", ")
	}
	lines := []string{
		"-- datadeck:export",
		"-- format_version: " + strconv.Itoa(m.FormatVersion),
		"-- engine: " + m.Engine,
		"-- database: " + m.Database,
		"-- mode: " + string(m.Mode),
		"-- generated_at: " + m.GeneratedAt.UTC().Format(time.RFC3339),
		"-- tables: " + strconv.Itoa(m.Tables),
		"-- indexes: " + strconv.Itoa(m.Indexes),
		"-- foreign_keys: " + strconv.Itoa(m.ForeignKeys),
	}
	if excluded != "" {
		lines = append(lines, "-- excluded: "+excluded)
	}
	_, err := io.WriteString(w, strings.Join(lines, "\n")+"\n\n")
	return err
}

// ReadHeader reads the leading comment block and returns the manifest. It
// returns ErrNotDataDeckArtifact when the datadeck marker is absent.
func ReadHeader(r *bufio.Reader) (Manifest, error) {
	var m Manifest
	seenMarker := false
	for {
		line, err := r.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(trimmed) == "" {
			if seenMarker {
				break // blank line terminates the header
			}
			if err != nil {
				return m, ErrNotDataDeckArtifact
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "--") {
			// Header ended without a blank line; leave the line unread is not
			// possible with ReadString, so treat a non-comment as a hard error
			// unless we already saw the marker (then it is the first statement,
			// already consumed — acceptable because our writer always emits a
			// blank separator).
			if !seenMarker {
				return m, ErrNotDataDeckArtifact
			}
			break
		}
		key, value, _ := strings.Cut(strings.TrimSpace(strings.TrimPrefix(trimmed, "--")), ":")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "datadeck":
			if value == "export" {
				seenMarker = true
			}
		case "format_version":
			m.FormatVersion, _ = strconv.Atoi(value)
		case "engine":
			m.Engine = value
		case "database":
			m.Database = value
		case "mode":
			m.Mode = Mode(value)
		case "generated_at":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				m.GeneratedAt = t
			}
		case "tables":
			m.Tables, _ = strconv.Atoi(value)
		case "indexes":
			m.Indexes, _ = strconv.Atoi(value)
		case "foreign_keys":
			m.ForeignKeys, _ = strconv.Atoi(value)
		case "excluded":
			if value != "" {
				m.Excluded = strings.Split(value, ", ")
			}
		}
		if err != nil {
			break
		}
	}
	if !seenMarker {
		return m, ErrNotDataDeckArtifact
	}
	return m, nil
}

// TableRef identifies one base table within the exported database.
type TableRef struct {
	Schema string // empty for engines without a schema level
	Name   string
}

// Source supplies the exporter with the database inventory, per-table structure
// and streamed row data. The handler implements it over the connection manager
// so this package stays free of database concerns.
type Source interface {
	Tables(ctx context.Context) ([]TableRef, error)
	Structure(ctx context.Context, ref TableRef) (model.TableStructure, error)
	// Data streams the table's rows to w as dialect INSERT statements.
	Data(ctx context.Context, ref TableRef, w io.Writer) error
}

// Export writes a SQL script for src to w. Ordering is deliberately
// dependency-free: every table is created without foreign keys, data is loaded,
// then foreign keys and secondary indexes are added — so circular and
// self-referencing foreign keys need no graph sorting and never break restore.
func Export(ctx context.Context, w io.Writer, manifest Manifest, src Source) error {
	tables, err := src.Tables(ctx)
	if err != nil {
		return err
	}

	structures := make([]model.TableStructure, 0, len(tables))
	refs := make([]TableRef, 0, len(tables))
	if manifest.Mode != ModeData {
		for _, ref := range tables {
			structure, err := src.Structure(ctx, ref)
			if err != nil {
				return fmt.Errorf("structure %s: %w", ref.label(), err)
			}
			structures = append(structures, structure)
			refs = append(refs, ref)
		}
	}

	manifest.Tables = len(tables)
	manifest.Indexes = 0
	manifest.ForeignKeys = 0
	for _, structure := range structures {
		manifest.Indexes += countSecondaryIndexes(structure)
		manifest.ForeignKeys += len(structure.ForeignKeys)
	}
	if err := WriteHeader(w, manifest); err != nil {
		return err
	}

	driver := manifest.Engine
	begin, commit := transactionControl(driver)

	bw := bufio.NewWriterSize(w, 64<<10)
	if _, err := fmt.Fprintln(bw, begin); err != nil {
		return err
	}

	if manifest.Mode != ModeData {
		for i, structure := range structures {
			if err := RenderCreateTable(bw, driver, refs[i], structure); err != nil {
				return err
			}
		}
	}

	if manifest.Mode != ModeSchema {
		for _, ref := range tables {
			if _, err := fmt.Fprintf(bw, "\n-- data for %s\n", ref.qualified(driver)); err != nil {
				return err
			}
			if err := src.Data(ctx, ref, bw); err != nil {
				return fmt.Errorf("data %s: %w", ref.label(), err)
			}
		}
	}

	if manifest.Mode != ModeData {
		for i, structure := range structures {
			if err := RenderIndexes(bw, driver, refs[i], structure); err != nil {
				return err
			}
			if driver != "sqlite" {
				if err := RenderForeignKeys(bw, driver, refs[i], structure); err != nil {
					return err
				}
			}
		}
	}

	if _, err := fmt.Fprintln(bw, commit); err != nil {
		return err
	}
	return bw.Flush()
}

// transactionControl returns the begin/commit pair. PostgreSQL and SQLite DDL is
// transactional, so the whole script is one transaction. MySQL DDL implicitly
// commits; its wrapper is presentational only (MySQL restore is disabled).
func transactionControl(driver string) (string, string) {
	if driver == "mysql" {
		return "START TRANSACTION;", "COMMIT;"
	}
	return "BEGIN;", "COMMIT;"
}

func (r TableRef) label() string {
	if r.Schema == "" {
		return r.Name
	}
	return r.Schema + "." + r.Name
}

func (r TableRef) qualified(driver string) string {
	quote := QuoteFor(driver)
	if r.Schema == "" || driver != "postgres" {
		return QuoteIdent(quote, r.Name)
	}
	return QuoteIdent(quote, r.Schema) + "." + QuoteIdent(quote, r.Name)
}
