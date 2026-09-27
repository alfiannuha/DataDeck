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
	// Dialect is the editor/introspection dialect identifier (e.g. "postgres").
	Dialect string `json:"dialect"`
	// IdentifierQuote is the character used to quote identifiers ("`" for
	// MySQL, `"` for PostgreSQL/SQLite).
	IdentifierQuote string `json:"identifier_quote"`
}
