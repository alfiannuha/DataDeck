package database

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// maxInValues bounds an IN list (ADR-010 §7: conservative maximum).
const maxInValues = 100

// filterCategory is a stable internal classification of a column's database
// type; operator compatibility is decided from it, never from frontend claims.
type filterCategory int

const (
	categoryUnsupported filterCategory = iota
	categoryText
	categoryEqualityOnly
	categoryNumeric
	categoryTemporal
	categoryBoolean
)

// textAndEqualityOperators apply to text-like columns.
func operatorsFor(category filterCategory) map[string]bool {
	eq := map[string]bool{
		"equals": true, "not_equals": true, "is_null": true, "is_not_null": true,
	}
	switch category {
	case categoryText:
		eq["contains"] = true
		eq["starts_with"] = true
		eq["ends_with"] = true
		eq["in"] = true
		return eq
	case categoryNumeric, categoryTemporal:
		eq["greater_than"] = true
		eq["greater_or_equal"] = true
		eq["less_than"] = true
		eq["less_or_equal"] = true
		eq["in"] = true
		return eq
	case categoryEqualityOnly:
		eq["in"] = true
		return eq
	default: // unsupported: null checks only
		return map[string]bool{"is_null": true, "is_not_null": true}
	}
}

// categorize maps a database type name to a stable category. Unknown types are
// treated conservatively (null checks only).
func categorize(dataType string) filterCategory {
	base := strings.ToLower(strings.TrimSpace(dataType))
	if i := strings.IndexAny(base, "( "); i >= 0 {
		base = base[:i]
	}
	switch base {
	case "char", "varchar", "nvarchar", "nchar", "character", "text", "clob",
		"tinytext", "mediumtext", "longtext", "citext", "enum", "set":
		return categoryText
	case "smallint", "int", "int2", "int4", "int8", "integer", "bigint",
		"serial", "bigserial", "smallserial", "mediumint", "tinyint", "decimal",
		"numeric", "real", "float", "float4", "float8", "double", "money":
		return categoryNumeric
	case "date", "time", "timestamp", "timestamptz", "datetime", "year":
		return categoryTemporal
	case "bool", "boolean":
		return categoryBoolean
	case "uuid", "uniqueidentifier":
		return categoryEqualityOnly
	default:
		return categoryUnsupported
	}
}

// filterClause builds a parameterized WHERE fragment for the given filters.
// Every column is resolved against canonical metadata and quoted; every user
// value is returned in args (never concatenated). Multiple filters are ANDed.
func filterClause(quote, driver string, filters []model.TableFilter, columns []model.Column) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	parts := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	for _, filter := range filters {
		column := canonicalColumn(columns, filter.Column)
		if column == nil {
			return "", nil, fmt.Errorf("%w: %s", ErrFilterColumnNotFound, filter.Column)
		}
		category := categorize(column.DataType)
		allowed := operatorsFor(category)
		if !allowed[filter.Operator] {
			return "", nil, fmt.Errorf("%w: operator %q is not valid for column %q", ErrInvalidFilter, filter.Operator, column.Name)
		}
		quoted := quoteIdentifier(quote, column.Name)

		switch filter.Operator {
		case "is_null", "is_not_null":
			if filter.Value != nil {
				return "", nil, fmt.Errorf("%w: %s does not take a value", ErrInvalidFilter, filter.Operator)
			}
			if filter.Operator == "is_null" {
				parts = append(parts, quoted+" IS NULL")
			} else {
				parts = append(parts, quoted+" IS NOT NULL")
			}
		case "in":
			if filter.Value != nil || len(filter.Values) == 0 {
				return "", nil, fmt.Errorf("%w: in requires values", ErrInvalidFilter)
			}
			if len(filter.Values) > maxInValues {
				return "", nil, fmt.Errorf("%w: in accepts at most %d values", ErrInvalidFilter, maxInValues)
			}
			placeholders := make([]string, len(filter.Values))
			for i, raw := range filter.Values {
				value, err := convertValue(category, raw)
				if err != nil {
					return "", nil, err
				}
				args = append(args, value)
				placeholders[i] = placeholder(driver, len(args))
			}
			parts = append(parts, quoted+" IN ("+strings.Join(placeholders, ", ")+")")
		case "contains", "starts_with", "ends_with":
			value, err := convertValue(category, filter.Value)
			if err != nil {
				return "", nil, err
			}
			text, ok := value.(string)
			if !ok {
				return "", nil, fmt.Errorf("%w: %s requires a text value", ErrInvalidFilter, filter.Operator)
			}
			pattern := escapeLike(text)
			switch filter.Operator {
			case "contains":
				pattern = "%" + pattern + "%"
			case "starts_with":
				pattern = pattern + "%"
			case "ends_with":
				pattern = "%" + pattern
			}
			args = append(args, pattern)
			parts = append(parts, quoted+" LIKE "+placeholder(driver, len(args))+" ESCAPE "+likeEscapeLiteral(driver))
		default:
			if filter.Value == nil {
				return "", nil, fmt.Errorf("%w: %s requires a value", ErrInvalidFilter, filter.Operator)
			}
			value, err := convertValue(category, filter.Value)
			if err != nil {
				return "", nil, err
			}
			args = append(args, value)
			parts = append(parts, quoted+" "+comparisonOperator(filter.Operator)+" "+placeholder(driver, len(args)))
		}
	}
	return strings.Join(parts, " AND "), args, nil
}

// comparisonOperator maps the validated enum to a SQL operator. It is a closed
// map (no caller-supplied text can reach it).
func comparisonOperator(operator string) string {
	switch operator {
	case "equals":
		return "="
	case "not_equals":
		return "<>"
	case "greater_than":
		return ">"
	case "greater_or_equal":
		return ">="
	case "less_than":
		return "<"
	case "less_or_equal":
		return "<="
	default:
		return "="
	}
}

// convertValue checks and normalizes a JSON scalar for the column category.
// Numbers are kept as exact strings (BIGINT safety); booleans stay booleans.
func convertValue(category filterCategory, raw any) (any, error) {
	switch value := raw.(type) {
	case nil:
		return nil, fmt.Errorf("%w: null requires is_null/is_not_null", ErrInvalidFilter)
	case json.Number:
		if category == categoryText || category == categoryTemporal || category == categoryBoolean {
			// Allow numeric text only for text columns; temporal/boolean are strict.
			if category == categoryText {
				return value.String(), nil
			}
			return nil, fmt.Errorf("%w: numeric value is not valid for this column", ErrInvalidFilter)
		}
		return value.String(), nil
	case string:
		if category == categoryBoolean {
			return nil, fmt.Errorf("%w: boolean column requires true/false", ErrInvalidFilter)
		}
		return value, nil
	case bool:
		if category != categoryBoolean {
			return nil, fmt.Errorf("%w: boolean value is not valid for this column", ErrInvalidFilter)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("%w: unsupported value type", ErrInvalidFilter)
	}
}

// escapeLike escapes LIKE metacharacters so user input is literal, using
// backslash with an explicit ESCAPE clause.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

// likeEscapeLiteral returns the SQL string literal for the ESCAPE character.
// MySQL processes backslashes in string literals, so it needs a doubled one.
func likeEscapeLiteral(driver string) string {
	if driver == string(model.DriverMySQL) {
		return `'\\'`
	}
	return `'\'`
}

// placeholder returns the driver's positional parameter marker.
func placeholder(driver string, n int) string {
	if driver == string(model.DriverPostgres) {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}
