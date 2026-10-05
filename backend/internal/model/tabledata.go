package model

// Table Data browsing models (PRF-02/ADR-010). They mirror the query-result
// serialization rules (BIGINT-as-string, NULL as null, bytea/base64, JSON
// embedded) because the same driver encoders produce the row values.

// TableSort is one server-side order expression (PRF-02/T05). Column is
// validated against introspection metadata and quoted by the dialect; Direction
// is restricted to the approved enum. Only the backend builds ORDER BY.
type TableSort struct {
	Column    string `json:"column" example:"created_at"`
	Direction string `json:"direction" enums:"asc,desc" example:"desc"`
}

// TableFilter is one structured server-side filter (PRF-02/T06). Column and
// Operator are validated against metadata/enums; values are bound parameters.
// Multiple filters are combined with AND. There is intentionally no field for
// raw SQL/WHERE. Numbers decode as json.Number so BIGINT text stays exact.
type TableFilter struct {
	Column   string `json:"column" example:"status"`
	Operator string `json:"operator" enums:"equals,not_equals,contains,starts_with,ends_with,greater_than,greater_or_equal,less_than,less_or_equal,is_null,is_not_null,in" example:"equals"`
	Value    any    `json:"value,omitempty" example:"active"`
	Values   []any  `json:"values,omitempty" example:"active,pending"`
}

// TableFilterEnvelope carries the decoded filter list (used by swagger docs).
type TableFilterEnvelope struct {
	Filters []TableFilter `json:"filters"`
}

// TableColumnInfo is the metadata needed to render and (later) mutate a column.
// It is derived from introspection, never trusted from the client.
type TableColumnInfo struct {
	Name            string `json:"name"`
	DatabaseType    string `json:"database_type"`
	Nullable        bool   `json:"nullable"`
	OrdinalPosition int    `json:"ordinal_position"`
	PrimaryKey      bool   `json:"primary_key"`
	// Insertable/Updatable are the effective per-column mutation flags
	// (PRF-02/T07): generated columns are neither; PK and identity columns are
	// not updatable in v0.1.0.
	Insertable bool `json:"insertable"`
	Updatable  bool `json:"updatable"`
	// HasDefault reports whether the column has a database default (drives the
	// Add Row DEFAULT default-mode; PRF-02/T08).
	HasDefault bool `json:"has_default"`
}

// RowCapabilities is the effective table-level mutation capability. Update/
// Delete require a declared primary key (v0.1.0); views/matviews/foreign tables
// are read-only.
type RowCapabilities struct {
	Insert    bool `json:"insert"`
	Update    bool `json:"update"`
	Delete    bool `json:"delete"`
	Duplicate bool `json:"duplicate"`
}

// RowIdentityInfo exposes the canonical row identity columns (primary key) so
// the frontend can build mutation requests from canonical metadata.
type RowIdentityInfo struct {
	Kind    string   `json:"kind"` // always "primary_key" in v0.1.0
	Columns []string `json:"columns"`
}

// InsertValue is one column's input for a single-row INSERT (PRF-02/T08). Mode
// is one of "value", "null", or "default"; Mode distinguishes an explicit NULL
// and the database DEFAULT from an empty string or the literal text "NULL"/
// "DEFAULT".
type InsertValue struct {
	Mode  string `json:"mode" enums:"value,null,default" example:"value"`
	Value any    `json:"value,omitempty" example:"Alfie"`
}

// RowMutationResult is the bounded outcome of a single-row mutation. Row is the
// canonical post-mutation row (metadata column order) when it can be read back.
type RowMutationResult struct {
	AffectedRows int   `json:"affected_rows"`
	Row          []any `json:"row,omitempty"`
}

// TablePagination is the bounded page descriptor. There is deliberately no
// total/total_pages: an exact COUNT(*) is not run for every browse request
// (ADR-010 §6).
type TablePagination struct {
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	HasMore  bool `json:"has_more"`
}

// TableDataPage is the structured response for GET /connections/{id}/table-data.
type TableDataPage struct {
	Database   string            `json:"database"`
	Schema     string            `json:"schema,omitempty"`
	Table      string            `json:"table"`
	ObjectType string            `json:"object_type"`
	Columns    []TableColumnInfo `json:"columns"`
	Rows       [][]any           `json:"rows"`
	Pagination TablePagination   `json:"pagination"`
	// Truncated is true when the shared 50 MB result cap stopped the page short.
	Truncated bool `json:"truncated"`
	// RowCapabilities/RowIdentity describe mutation eligibility derived from
	// canonical metadata (PRF-02/T07). Update/Delete require a primary key.
	RowCapabilities RowCapabilities  `json:"row_capabilities"`
	RowIdentity     *RowIdentityInfo `json:"row_identity,omitempty"`
}
