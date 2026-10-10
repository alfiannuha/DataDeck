// Package export implements the table-export sinks (CSV, SQL, XLSX) for
// PRF-02/T14. Sinks implement database.ExportSink structurally: they import
// only the model package (never database), so the database layer stays free of
// encoding concerns and there is no import cycle.
//
// Row values are the canonical driver-encoded model values described in
// docs/api-contract.md (BIGINT/DECIMAL/uuid/timestamps as strings, NULL as nil,
// JSON as json.RawMessage, bytea/blobs as base64). Every sink renders those
// values without re-serializing them: big integers and decimals stay exact,
// JSON stays embedded JSON, timestamps stay RFC 3339 text.
package export

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// cellString converts a canonical row value into the text the CSV/SQL sinks
// write. It returns ("", true) for NULL so callers can emit their dialect's
// empty field / NULL literal instead of the string "null".
func cellString(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", true
	case string:
		return v, false
	case json.RawMessage:
		return string(v), false
	case bool:
		if v {
			return "true", false
		}
		return "false", false
	case time.Time:
		return v.Format(time.RFC3339Nano), false
	case fmt.Stringer:
		return v.String(), false
	case error:
		return v.Error(), false
	case []byte:
		return string(v), false
	default:
		return fmt.Sprintf("%v", v), false
	}
}

// guardFormula neutralizes spreadsheet formula-injection leading characters
// (=, +, @, -). A leading "-" that begins a plain negative number (e.g. "-42")
// is left untouched because it is not a formula vector. The behaviour mirrors
// the frontend guardFormulaExport canonical source on purpose so CSV exports
// and the Query Results CSV stay consistent.
func guardFormula(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '@', '-':
		if firstCharIsDigitAfterOptionalMinus(value) {
			return value
		}
		return "'" + value
	}
	return value
}

// firstCharIsDigitAfterOptionalMinus reports whether value is a plain number
// such as "-42" or "123" (no separator run beyond the leading minus).
func firstCharIsDigitAfterOptionalMinus(value string) bool {
	i := 0
	if value[0] == '-' {
		i = 1
		if len(value) == 1 {
			return false
		}
	}
	c := value[i]
	return c >= '0' && c <= '9'
}

// baseType normalizes a database type name: lowercase, cutting any length/
// precision suffix ("varchar(255)", "DECIMAL(10,2)") and any ANSI qualifier
// like "character varying" down to the leading keyword.
func baseType(databaseType string) string {
	name := strings.ToLower(strings.TrimSpace(databaseType))
	if i := strings.IndexAny(name, "( "); i >= 0 {
		name = name[:i]
	}
	return name
}

// quoteIdentifier quotes one identifier with the engine's quote character,
// doubling embedded quote characters (as the database layer already does).
func quoteIdentifier(quote, identifier string) string {
	return quote + strings.ReplaceAll(identifier, quote, quote+quote) + quote
}

// typeOfColumn returns the metadata-driven (not runtime-derived) SQL type of a
// sink column so SQL scripts stay dialect-correct even before the first row.
func typeOfColumn(column model.QueryColumn) string {
	return baseType(column.Type)
}