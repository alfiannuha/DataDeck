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
}
