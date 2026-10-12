package database

import (
	"strings"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// structure.go holds driver-neutral helpers used by every engine's
// TableStructure implementation. The mutability rules are the single source of
// truth shared with Table Data (columnInfo): a generated column is neither
// insertable nor updatable, and a primary-key / identity column is not
// updatable in v0.1.0. Keeping this derivation here guarantees the structure
// endpoint never contradicts the mutation endpoint.
func structureColumnMutability(generated, identity, primaryKey bool) (insertable, updatable bool) {
	insertable = !generated && !identity
	updatable = insertable && !primaryKey
	return insertable, updatable
}

// normalizedType strips engine-native decorations (length, precision, PG "with
// time zone" tails) and collapses common aliases to a short label. It is
// optional ("if available") and conservative: anything unrecognized is returned
// as the stripped native base so no type is ever fabricated.
func normalizedType(dataType string) string {
	base := strings.ToLower(strings.TrimSpace(dataType))
	if i := strings.IndexByte(base, '('); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	aliases := map[string]string{
		"character varying":           "varchar",
		"varchar":                     "varchar",
		"nvarchar":                    "varchar",
		"character":                   "char",
		"nchar":                       "char",
		"char":                        "char",
		"bpchar":                      "char",
		"text":                        "text",
		"tinytext":                    "text",
		"mediumtext":                  "text",
		"longtext":                    "text",
		"clob":                        "text",
		"citext":                      "text",
		"int2":                        "smallint",
		"smallint":                    "smallint",
		"int":                         "integer",
		"int4":                        "integer",
		"integer":                     "integer",
		"mediumint":                   "integer",
		"int8":                        "bigint",
		"bigint":                      "bigint",
		"serial":                      "integer",
		"bigserial":                   "bigint",
		"smallserial":                 "smallint",
		"decimal":                     "numeric",
		"numeric":                     "numeric",
		"money":                       "numeric",
		"real":                        "real",
		"float4":                      "real",
		"float":                       "double",
		"float8":                      "double",
		"double precision":            "double",
		"double":                      "double",
		"boolean":                     "boolean",
		"bool":                        "boolean",
		"date":                        "date",
		"time":                        "time",
		"timetz":                      "time",
		"timestamp":                   "timestamp",
		"timestamp without time zone": "timestamp",
		"timestamp with time zone":    "timestamp",
		"timestamptz":                 "timestamp",
		"datetime":                    "datetime",
		"year":                        "integer",
		"uuid":                        "uuid",
		"uniqueidentifier":            "uuid",
		"json":                        "json",
		"jsonb":                       "json",
		"bytea":                       "bytea",
		"blob":                        "bytea",
		"varbinary":                   "bytea",
		"binary":                      "bytea",
		"enum":                        "enum",
		"set":                         "set",
	}
	if normalized, ok := aliases[base]; ok {
		return normalized
	}
	return base
}

// structureColumnMutabilityForTable applies the table-level editability rule on
// top of the pure column rule: views, materialized views and foreign tables are
// read-only in v0.1.0, so none of their columns are insertable or updatable.
func structureColumnMutabilityForTable(objectType string, columns []model.StructureColumn) {
	if isBaseTable(objectType) {
		return
	}
	for i := range columns {
		columns[i].Insertable = false
		columns[i].Updatable = false
	}
}
