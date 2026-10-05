package model

// Schema introspection models. These are database-neutral: every target driver
// populates the same structures, so the API and the frontend schema explorer do
// not depend on a specific engine's catalog shape.

// Database is the root of an introspection tree. Engines without a schema
// level (e.g. SQLite, MySQL) place tables directly in Tables and leave Schemas
// empty; engines with schemas (PostgreSQL) populate Schemas. This lets the
// same model represent different hierarchy depths without inventing fake
// schemas.
type Database struct {
	Name    string   `json:"name"`
	Schemas []Schema `json:"schemas,omitempty"`
	Tables  []Table  `json:"tables,omitempty"`
}

// Schema is a namespace inside a database.
type Schema struct {
	Name   string  `json:"name"`
	Tables []Table `json:"tables"`
}

// Table holds a relation and its structural metadata.
type Table struct {
	Schema      string       `json:"schema"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Columns     []Column     `json:"columns"`
	PrimaryKey  *PrimaryKey  `json:"primary_key,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty"`
}

// Column describes a single table column.
type Column struct {
	Name            string  `json:"name"`
	DataType        string  `json:"data_type"`
	Nullable        bool    `json:"nullable"`
	Default         *string `json:"default,omitempty"`
	OrdinalPosition int     `json:"ordinal_position"`
	// Generated marks computed/generated columns (never directly writable) where
	// the engine exposes this metadata (PRF-02/T07).
	Generated bool `json:"generated,omitempty"`
	// Identity marks engine-assigned columns (identity/auto-increment/rowid
	// alias): excluded from Update, and from explicit Insert values in v0.1.0.
	Identity bool `json:"identity,omitempty"`
}

// PrimaryKey describes a table's primary key.
type PrimaryKey struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// ForeignKey describes a foreign key constraint, column-by-column.
type ForeignKey struct {
	Name              string   `json:"name"`
	Columns           []string `json:"columns"`
	ReferencedSchema  string   `json:"referenced_schema"`
	ReferencedTable   string   `json:"referenced_table"`
	ReferencedColumns []string `json:"referenced_columns"`
}

// Index describes an index on a table. Expression index parts are represented
// as an empty column name.
type Index struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
	Columns []string `json:"columns"`
}

// DatabaseInfo is lightweight metadata for one selectable database on a
// server-level connection (PRF-01). It deliberately carries no schema/tables:
// discovery must stay cheap and must not imply that schemas were loaded.
type DatabaseInfo struct {
	Name string `json:"name"`
	// BootstrapCandidate marks the conventional maintenance database (e.g.
	// PostgreSQL "postgres") used when a profile has no default database.
	BootstrapCandidate bool `json:"bootstrap_candidate,omitempty"`
}
