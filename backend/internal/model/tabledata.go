package model

// Table Data browsing models (PRF-02/ADR-010). They mirror the query-result
// serialization rules (BIGINT-as-string, NULL as null, bytea/base64, JSON
// embedded) because the same driver encoders produce the row values.

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
