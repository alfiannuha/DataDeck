package model

// Capabilities describes driver-level feature differences that the engine and
// UI need to reason about. Only capabilities DataDeck currently needs (or needs
// immediately for M3) are represented — no speculative enterprise features.
type Capabilities struct {
	// Schemas reports whether the engine has a schema level between the
	// database and its tables (PostgreSQL: yes; MySQL/SQLite: no).
	Schemas bool `json:"schemas"`
	// ForeignKeys / Indexes report whether introspection provides this metadata.
	ForeignKeys bool `json:"foreign_keys"`
	Indexes     bool `json:"indexes"`
	// SSL / SSH report whether connection settings for these are meaningful.
	SSL bool `json:"ssl"`
	SSH bool `json:"ssh"`
	// MultipleDatabases reports whether the engine exposes multiple selectable
	// databases on one server-level connection (PostgreSQL: true; MySQL/SQLite:
	// false — PRF-01).
	MultipleDatabases bool `json:"multiple_databases"`
	// Dialect is the editor/introspection dialect identifier (e.g. "postgres").
	Dialect string `json:"dialect"`
	// IdentifierQuote is the character used to quote identifiers ("`" for
	// MySQL, `"` for PostgreSQL/SQLite).
	IdentifierQuote string `json:"identifier_quote"`
	// TableExport reports whether the driver streams table exports and which
	// formats it supports (PRF-02/T14). All three drivers advertise it in
	// v0.1.0; per-object applicability (e.g. SQL export on a view) is derived
	// from introspection metadata, not from this flag.
	TableExport   bool     `json:"table_export"`
	ExportFormats []string `json:"export_formats"`
	// TableImport reports whether the driver validates and imports table data
	// from uploaded files and which formats it supports (PRF-02/T15): only
	// value-level INSERTs for a mutable base table, always parameterized.
	TableImport   bool     `json:"table_import"`
	ImportFormats []string `json:"import_formats"`
}
