package model

import "time"

// Table Import models (PRF-02/T15, ADR-010 §13/§14). A table import is a
// session-bound, value-only operation: the client uploads CSV (RFC 4180) or a
// restricted SQL INSERT-only script, the backend parses it into cells, maps
// source columns to target columns, validates values against metadata, and
// executes parameterized INSERTs in one all-or-nothing transaction. There is
// never a raw SQL/expression path and never a partial commit.

// ImportCell is one parsed value from a source file: a literal string, or NULL.
// The CSV parser resolves the quoted/unquoted distinction up front (unquoted
// empty → NULL; quoted "" → empty string, ADR-010 §13).
type ImportCell struct {
	// IsNull reports an explicit NULL (empty unquoted CSV field, or the NULL
	// literal in an SQL script).
	IsNull bool   `json:"is_null"`
	Value  string `json:"value,omitempty"`
}

// ImportMapping binds one source column (0-based index) to a target column.
// Leaving mapping empty requests auto-mapping by column name; the backend
// always resolves and validates the final mapping, never the client.
type ImportMapping struct {
	Source int    `json:"source" example:"0"`
	Target string `json:"target" example:"name"`
}

// TableImportSession is the immutable binding for one upload: the resolved
// target relation plus the file fingerprint (TOCTOU guard) and parsed source
// columns. Created once by the upload endpoint; validate/execute echo the
// target, which must exactly match the frozen binding.
type TableImportSession struct {
	ImportID     string         `json:"import_id"`
	ConnectionID string         `json:"-"`
	Database     string         `json:"database"`
	Schema       string         `json:"schema,omitempty"`
	Table        string         `json:"table"`
	Format       string         `json:"format"`
	Filename     string         `json:"filename"`
	SizeBytes    int64          `json:"size_bytes"`
	SHA256       string         `json:"-"`
	HeaderRow    bool           `json:"header_row"`
	Columns      []string       `json:"columns"`
	Preview      [][]ImportCell `json:"preview"`
	RowCount     int            `json:"row_count"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ImportMappingResolved is one backend-resolved source→target binding.
type ImportMappingResolved struct {
	Source int    `json:"source"`
	Target string `json:"target"`
}

// ImportValidationError is one sanitized, localized validation finding.
type ImportValidationError struct {
	Row     int    `json:"row" example:"17"`
	Column  string `json:"column,omitempty" example:"age"`
	Message string `json:"message" example:"value is not numeric"`
}

// TableImportValidation is the validate-stage response: the resolved mapping,
// the per-column plan, and the first N findings. No data is written.
type TableImportValidation struct {
	Database   string                  `json:"database"`
	Schema     string                  `json:"schema,omitempty"`
	Table      string                  `json:"table"`
	ObjectType string                  `json:"object_type"`
	Columns    []TableColumnInfo       `json:"columns"`
	Mapping    []ImportMappingResolved `json:"mapping"`
	RowCount   int                     `json:"row_count"`
	Valid      bool                    `json:"valid"`
	Errors     []ImportValidationError `json:"errors"`
}

// TableImportExecutionResult is the outcome of the execute stage.
type TableImportExecutionResult struct {
	Database  string `json:"database"`
	Schema    string `json:"schema,omitempty"`
	Table     string `json:"table"`
	Inserted  int    `json:"inserted"`
	RowCount  int    `json:"rows"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// ImportPlan is the resolved target + mapping computed by the database layer
// from canonical metadata; conversion and execution both flow through it.
type ImportPlan struct {
	Schema     string
	Table      string
	ObjectType string
	BaseTable  bool
	// SourceCount is the number of source columns (file width).
	SourceCount int
	// Columns are the insertable columns in metadata ordinal order.
	Columns []Column
	// ColumnInfo mirrors Columns for API responses (Insertable/Name/type/…).
	ColumnInfo []TableColumnInfo
	Mapping    []ImportMappingResolved
}
