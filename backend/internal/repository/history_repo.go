package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// DefaultHistoryLimit caps how many history records a List call returns when no
// explicit limit is supplied.
const DefaultHistoryLimit = 100

// HistoryFilter narrows a history listing. A zero ConnectionID means "all
// connections"; a Limit <= 0 falls back to DefaultHistoryLimit and a negative
// Offset is treated as 0.
type HistoryFilter struct {
	ConnectionID string
	Limit        int
	Offset       int
}

// QueryHistoryRepository persists query execution audit records. It never
// executes target-database SQL; it only records metadata about executions.
type QueryHistoryRepository struct {
	db *sql.DB
}

// NewQueryHistoryRepository returns a repository backed by db.
func NewQueryHistoryRepository(db *sql.DB) *QueryHistoryRepository {
	return &QueryHistoryRepository{db: db}
}

const historyColumns = `id, connection_id, database_name, sql_text, status,
	execution_time_ms, rows_affected, error_message, executed_at`

// Create inserts an audit record, assigning ExecutedAt when unset. A missing
// referenced connection is reported as ErrNotFound (foreign key violation).
func (r *QueryHistoryRepository) Create(ctx context.Context, history *model.QueryHistory) error {
	if history == nil {
		return errors.New("history repository: history must not be nil")
	}
	if history.ExecutedAt.IsZero() {
		history.ExecutedAt = time.Now().UTC()
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO query_history (`+historyColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		history.ID, history.ConnectionID, nullString(history.DatabaseName), history.SQLText,
		string(history.Status), history.ExecutionTimeMS, history.RowsAffected,
		nullString(history.ErrorMessage), history.ExecutedAt)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("connection %q: %w", history.ConnectionID, ErrNotFound)
		}
		if isUniqueViolation(err) {
			return fmt.Errorf("history %q: %w", history.ID, ErrConflict)
		}
		return fmt.Errorf("create history %q: %w", history.ID, err)
	}
	return nil
}

// List returns history records newest-first. Results are ordered by
// executed_at then id, both descending, so ordering is deterministic even for
// records sharing a timestamp.
func (r *QueryHistoryRepository) List(ctx context.Context, filter HistoryFilter) ([]model.QueryHistory, error) {
	query := `SELECT ` + historyColumns + ` FROM query_history`
	args := make([]any, 0, 2)
	if filter.ConnectionID != "" {
		query += ` WHERE connection_id = ?`
		args = append(args, filter.ConnectionID)
	}
	query += ` ORDER BY executed_at DESC, id DESC LIMIT ? OFFSET ?`

	limit := filter.Limit
	if limit <= 0 {
		limit = DefaultHistoryLimit
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	records := make([]model.QueryHistory, 0)
	for rows.Next() {
		var (
			history  model.QueryHistory
			status   string
			errorMsg sql.NullString
		)
		var database sql.NullString
		if err := rows.Scan(&history.ID, &history.ConnectionID, &database, &history.SQLText, &status,
			&history.ExecutionTimeMS, &history.RowsAffected, &errorMsg, &history.ExecutedAt); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		history.Status = model.QueryStatus(status)
		history.DatabaseName = stringPtr(database)
		history.ErrorMessage = stringPtr(errorMsg)
		records = append(records, history)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history: %w", err)
	}
	return records, nil
}

// Count returns the total number of records matching the filter (ignoring
// Limit/Offset), for pagination metadata.
func (r *QueryHistoryRepository) Count(ctx context.Context, filter HistoryFilter) (int, error) {
	query := `SELECT COUNT(*) FROM query_history`
	args := make([]any, 0, 1)
	if filter.ConnectionID != "" {
		query += ` WHERE connection_id = ?`
		args = append(args, filter.ConnectionID)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count history: %w", err)
	}
	return total, nil
}
