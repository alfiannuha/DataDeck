package model

import "time"

// QueryStatus is the recorded outcome of a query execution.
type QueryStatus string

const (
	QueryStatusSuccess QueryStatus = "SUCCESS"
	QueryStatusError   QueryStatus = "ERROR"
)

// QueryHistory is a persisted audit record of one query execution.
type QueryHistory struct {
	ID              string
	ConnectionID    string
	SQLText         string
	Status          QueryStatus
	ExecutionTimeMS int64
	RowsAffected    int64
	ErrorMessage    *string
	ExecutedAt      time.Time
}

// SavedQuery is a persisted SQL snippet. ConnectionID is optional (a snippet
// may be unbound); it is a pointer to preserve that distinction.
type SavedQuery struct {
	ID           string
	ConnectionID *string
	Title        string
	SQLText      string
	Tags         *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// QueryColumn describes one result column.
type QueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// QueryResult is the outcome of executing a statement. Rows are positional
// arrays aligned with Columns; 64-bit integers are serialized as strings and
// binary values as base64 (see the PostgreSQL encoder).
type QueryResult struct {
	Columns         []QueryColumn `json:"columns"`
	Rows            [][]any       `json:"rows"`
	RowsAffected    int64         `json:"rows_affected"`
	ExecutionTimeMS int64         `json:"execution_time_ms"`
	Truncated       bool          `json:"truncated"`
}
