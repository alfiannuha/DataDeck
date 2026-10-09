package model

// TableStructure is the canonical read-only structure of a single relation
// (PRF-02/T13). It is always bound to an explicit connection + database +
// schema + table, mirroring the metadata semantics already used by Table Data
// (PRF-02/T07) so structure and mutation rules never contradict each other
// (same declared-primary-key definition, same generated/identity flags, same
// insertable/updatable derivation).
type TableStructure struct {
	Database    string                `json:"database"`
	Schema      string                `json:"schema,omitempty"`
	Table       string                `json:"table"`
	ObjectType  string                `json:"object_type"`
	Columns     []StructureColumn     `json:"columns"`
	PrimaryKey  *PrimaryKey           `json:"primary_key,omitempty"`
	Constraints []StructureConstraint `json:"constraints,omitempty"`
	Indexes     []StructureIndex      `json:"indexes,omitempty"`
	ForeignKeys []StructureForeignKey `json:"foreign_keys,omitempty"`
}

// StructureColumn is the canonical column metadata for the structure endpoint.
// Insertable/Updatable reuse the exact derivation used by Table Data (T07):
// generated columns are neither; PK and identity columns are not updatable in
// v0.1.0. Optional fields (length/precision/scale/collation/comment) are only
// populated when the engine exposes them reliably.
type StructureColumn struct {
	Name               string  `json:"name"`
	OrdinalPosition    int     `json:"ordinal_position"`
	DatabaseType       string  `json:"database_type"`
	NormalizedType     string  `json:"normalized_type,omitempty"`
	Nullable           bool    `json:"nullable"`
	Default            *string `json:"default,omitempty"`
	PrimaryKey         bool    `json:"primary_key"`
	PrimaryKeyPosition int     `json:"primary_key_position,omitempty"`
	Generated          bool    `json:"generated,omitempty"`
	Identity           bool    `json:"identity,omitempty"`
	Insertable         bool    `json:"insertable"`
	Updatable          bool    `json:"updatable"`
	Length             int     `json:"length,omitempty"`
	Precision          int     `json:"precision,omitempty"`
	Scale              int     `json:"scale,omitempty"`
	Collation          string  `json:"collation,omitempty"`
	Comment            string  `json:"comment,omitempty"`
}

// StructureIndex is one index on the relation, in canonical column order.
// Expression/partial index parts use the schema-tree convention of an empty
// column name; Method carries the engine's index type when reliable.
type StructureIndex struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
	Method  string   `json:"method,omitempty"`
	Partial bool     `json:"partial,omitempty"`
	Columns []string `json:"columns"`
}

// StructureConstraint is one table-level constraint. Type is a stable label
// (PRIMARY KEY | UNIQUE | CHECK | FOREIGN KEY); Definition holds the expression
// text for CHECK/UNIQUE when the engine exposes it and is never rendered as
// HTML and never executed. NOT NULL is a column attribute, not a constraint.
type StructureConstraint struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Columns    []string `json:"columns,omitempty"`
	Definition string   `json:"definition,omitempty"`
}

// StructureForeignKey is one foreign key constraint with its referential
// actions, preserving composite column ordering.
type StructureForeignKey struct {
	Name              string   `json:"name"`
	Columns           []string `json:"columns"`
	ReferencedSchema  string   `json:"referenced_schema,omitempty"`
	ReferencedTable   string   `json:"referenced_table"`
	ReferencedColumns []string `json:"referenced_columns"`
	OnUpdate          string   `json:"on_update,omitempty"`
	OnDelete          string   `json:"on_delete,omitempty"`
}
