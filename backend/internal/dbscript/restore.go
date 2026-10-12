package dbscript

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// MaxRestoreBytes bounds a restore artifact. It is read fully for validation so
// the whole script is checked before any statement runs; CSV/SQL dumps of local
// developer databases stay well below this.
const MaxRestoreBytes = 256 << 20 // 256 MiB

// ErrUnsupportedStatement marks a statement outside the restore allowlist.
var ErrUnsupportedStatement = errors.New("dbscript: unsupported statement")

// ErrManifestMismatch marks an artifact whose engine/database does not match the
// explicit restore target.
var ErrManifestMismatch = errors.New("dbscript: artifact does not match the restore target")

// StatementKind is the allowlisted operation class of a restore statement.
type StatementKind string

const (
	StmtCreateTable StatementKind = "CREATE TABLE"
	StmtCreateIndex StatementKind = "CREATE INDEX"
	StmtAlterTable  StatementKind = "ALTER TABLE"
	StmtInsert      StatementKind = "INSERT"
	StmtTransaction StatementKind = "TRANSACTION"
	StmtPRAGMA      StatementKind = "PRAGMA"
)

// Statement is one validated, allowlisted statement from a restore artifact.
type Statement struct {
	Kind StatementKind
	SQL  string
}

// ValidateManifest binds an artifact to the explicit restore target. It never
// allows the artifact to redirect the operation to another engine or database.
func ValidateManifest(m Manifest, engine, database string) error {
	if m.FormatVersion != FormatVersion {
		return fmt.Errorf("%w: unsupported format version %d", ErrManifestMismatch, m.FormatVersion)
	}
	if !strings.EqualFold(m.Engine, engine) {
		return fmt.Errorf("%w: artifact engine %q, target engine %q", ErrManifestMismatch, m.Engine, engine)
	}
	if m.Database != database {
		return fmt.Errorf("%w: artifact database %q, target database %q", ErrManifestMismatch, m.Database, database)
	}
	return nil
}

// ParseStatements reads a bounded artifact body (header already consumed) and
// returns the allowlisted statements in order. Every statement is classified;
// any statement outside the allowlist rejects the whole artifact. It is not a
// general SQL parser: it deliberately accepts only the exact operation classes
// this exporter emits.
func ParseStatements(r io.Reader) ([]Statement, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(MaxRestoreBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxRestoreBytes {
		return nil, fmt.Errorf("artifact exceeds the %d byte limit", MaxRestoreBytes)
	}

	segments := splitStatements(string(data))
	statements := make([]Statement, 0, len(segments))
	for _, segment := range segments {
		text := strings.TrimSpace(stripLeadingComments(segment))
		if text == "" {
			continue
		}
		kind, err := classifyStatement(text)
		if err != nil {
			return nil, err
		}
		statements = append(statements, Statement{Kind: kind, SQL: text})
	}
	return statements, nil
}

// stripLeadingComments removes leading whitespace and -- / /* */ comments so a
// segment that begins with a generated "-- data for …" comment classifies as
// the statement that follows it.
func stripLeadingComments(text string) string {
	for {
		text = strings.TrimLeft(text, " \t\r\n")
		switch {
		case strings.HasPrefix(text, "--"):
			if idx := strings.IndexByte(text, '\n'); idx >= 0 {
				text = text[idx+1:]
				continue
			}
			return ""
		case strings.HasPrefix(text, "/*"):
			if idx := strings.Index(text, "*/"); idx >= 0 {
				text = text[idx+2:]
				continue
			}
			return ""
		default:
			return text
		}
	}
}

// classifyStatement maps a statement to an allowlisted kind or rejects it.
func classifyStatement(text string) (StatementKind, error) {
	upper := strings.ToUpper(text)
	switch {
	case strings.HasPrefix(upper, "CREATE TABLE"):
		// Never a CREATE TABLE ... AS SELECT (would execute a query).
		if strings.Contains(upper, " AS SELECT") || strings.Contains(upper, " AS\nSELECT") {
			return "", fmt.Errorf("%w: CREATE TABLE AS is not allowed", ErrUnsupportedStatement)
		}
		return StmtCreateTable, nil
	case strings.HasPrefix(upper, "CREATE UNIQUE INDEX"), strings.HasPrefix(upper, "CREATE INDEX"):
		return StmtCreateIndex, nil
	case strings.HasPrefix(upper, "ALTER TABLE"):
		// Only additive constraint clauses are emitted by the exporter.
		if !strings.Contains(upper, " ADD ") {
			return "", fmt.Errorf("%w: only ALTER TABLE ... ADD CONSTRAINT is allowed", ErrUnsupportedStatement)
		}
		return StmtAlterTable, nil
	case strings.HasPrefix(upper, "INSERT INTO"), strings.HasPrefix(upper, "INSERT "):
		return StmtInsert, nil
	case upper == "BEGIN", upper == "BEGIN;", strings.HasPrefix(upper, "BEGIN "),
		strings.HasPrefix(upper, "START TRANSACTION"), upper == "COMMIT", upper == "COMMIT;":
		return StmtTransaction, nil
	case strings.HasPrefix(upper, "PRAGMA"):
		return StmtPRAGMA, nil
	default:
		first := upper
		if idx := strings.IndexAny(upper, " \n\t;("); idx >= 0 {
			first = upper[:idx]
		}
		return "", fmt.Errorf("%w: %q", ErrUnsupportedStatement, first)
	}
}

// splitStatements splits on ';' while respecting single-quoted strings,
// double-quoted/quoted identifiers, backtick identifiers, and -- / /* */ comments.
// It never splits inside a literal or comment.
func splitStatements(data string) []string {
	var (
		segments []string
		b        strings.Builder
		i        int
	)
	for i < len(data) {
		c := data[i]
		switch {
		case c == '\'':
			j := scanQuoted(data, i, '\'')
			b.WriteString(data[i:j])
			i = j
		case c == '"':
			j := scanQuoted(data, i, '"')
			b.WriteString(data[i:j])
			i = j
		case c == '`':
			j := scanQuoted(data, i, '`')
			b.WriteString(data[i:j])
			i = j
		case c == '-' && i+1 < len(data) && data[i+1] == '-':
			j := strings.IndexByte(data[i:], '\n')
			if j < 0 {
				b.WriteString(data[i:])
				i = len(data)
			} else {
				b.WriteString(data[i : i+j])
				i += j
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			end := strings.Index(data[i+2:], "*/")
			if end < 0 {
				b.WriteString(data[i:])
				i = len(data)
			} else {
				j := i + 2 + end + 2
				b.WriteString(data[i:j])
				i = j
			}
		case c == ';':
			segments = append(segments, b.String())
			b.Reset()
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	segments = append(segments, b.String())
	return segments
}

// scanQuoted returns the index just past a quoted region starting at start.
func scanQuoted(data string, start int, quote byte) int {
	i := start + 1
	for i < len(data) {
		if data[i] == quote {
			if i+1 < len(data) && data[i+1] == quote { // doubled quote escape
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return len(data) // unterminated: treat the rest as one region
}
