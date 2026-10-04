package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// SavedQueryRepository persists saved SQL snippets. A snippet may exist without
// an associated connection.
type SavedQueryRepository struct {
	db *sql.DB
}

// NewSavedQueryRepository returns a repository backed by db.
func NewSavedQueryRepository(db *sql.DB) *SavedQueryRepository {
	return &SavedQueryRepository{db: db}
}

const savedQueryColumns = `id, connection_id, database_name, title, sql_text, tags, created_at, updated_at`

// Create inserts a snippet, assigning timestamps when unset. A non-nil
// ConnectionID that does not exist is reported as ErrNotFound.
func (r *SavedQueryRepository) Create(ctx context.Context, query *model.SavedQuery) error {
	if query == nil {
		return errors.New("saved query repository: query must not be nil")
	}
	now := time.Now().UTC()
	if query.CreatedAt.IsZero() {
		query.CreatedAt = now
	}
	if query.UpdatedAt.IsZero() {
		query.UpdatedAt = query.CreatedAt
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO saved_queries (`+savedQueryColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		query.ID, nullString(query.ConnectionID), nullString(query.DatabaseName),
		query.Title, query.SQLText, nullString(query.Tags), query.CreatedAt, query.UpdatedAt)
	if err != nil {
		if isForeignKeyViolation(err) {
			if query.ConnectionID != nil {
				return fmt.Errorf("connection %q: %w", *query.ConnectionID, ErrNotFound)
			}
			return fmt.Errorf("create saved query %q: %w", query.ID, ErrNotFound)
		}
		if isUniqueViolation(err) {
			return fmt.Errorf("saved query %q: %w", query.ID, ErrConflict)
		}
		return fmt.Errorf("create saved query %q: %w", query.ID, err)
	}
	return nil
}

// Get returns the snippet with the given id, or ErrNotFound.
func (r *SavedQueryRepository) Get(ctx context.Context, id string) (*model.SavedQuery, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+savedQueryColumns+` FROM saved_queries WHERE id = ?`, id)
	query, err := scanSavedQuery(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("saved query %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get saved query %q: %w", id, err)
	}
	return query, nil
}

// DefaultSavedQueryLimit caps an unbounded saved-query listing.
const DefaultSavedQueryLimit = 50

// List returns snippets ordered by updated_at then id, both descending, bounded
// by limit/offset.
func (r *SavedQueryRepository) List(ctx context.Context, limit, offset int) ([]model.SavedQuery, error) {
	limit, offset = normalizePagination(limit, offset, DefaultSavedQueryLimit)
	return r.query(ctx,
		`SELECT `+savedQueryColumns+` FROM saved_queries ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`,
		limit, offset)
}

// ListByConnection returns snippets bound to a connection, newest-updated first,
// bounded by limit/offset.
func (r *SavedQueryRepository) ListByConnection(ctx context.Context, connectionID string, limit, offset int) ([]model.SavedQuery, error) {
	limit, offset = normalizePagination(limit, offset, DefaultSavedQueryLimit)
	return r.query(ctx,
		`SELECT `+savedQueryColumns+` FROM saved_queries WHERE connection_id = ? ORDER BY updated_at DESC, id DESC LIMIT ? OFFSET ?`,
		connectionID, limit, offset)
}

// Count returns the total number of snippets.
func (r *SavedQueryRepository) Count(ctx context.Context) (int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM saved_queries`).Scan(&total); err != nil {
		return 0, fmt.Errorf("count saved queries: %w", err)
	}
	return total, nil
}

// CountByConnection returns the number of snippets bound to a connection.
func (r *SavedQueryRepository) CountByConnection(ctx context.Context, connectionID string) (int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM saved_queries WHERE connection_id = ?`, connectionID).Scan(&total); err != nil {
		return 0, fmt.Errorf("count saved queries: %w", err)
	}
	return total, nil
}

func normalizePagination(limit, offset, fallback int) (int, int) {
	if limit <= 0 {
		limit = fallback
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// Update replaces the mutable fields of an existing snippet and refreshes
// UpdatedAt. It returns ErrNotFound if no row matches.
func (r *SavedQueryRepository) Update(ctx context.Context, query *model.SavedQuery) error {
	if query == nil {
		return errors.New("saved query repository: query must not be nil")
	}
	query.UpdatedAt = time.Now().UTC()

	res, err := r.db.ExecContext(ctx,
		`UPDATE saved_queries SET connection_id = ?, database_name = ?, title = ?, sql_text = ?, tags = ?, updated_at = ? WHERE id = ?`,
		nullString(query.ConnectionID), nullString(query.DatabaseName), query.Title,
		query.SQLText, nullString(query.Tags), query.UpdatedAt, query.ID)
	if err != nil {
		if isForeignKeyViolation(err) {
			if query.ConnectionID != nil {
				return fmt.Errorf("connection %q: %w", *query.ConnectionID, ErrNotFound)
			}
			return fmt.Errorf("update saved query %q: %w", query.ID, ErrNotFound)
		}
		return fmt.Errorf("update saved query %q: %w", query.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update saved query %q: %w", query.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("saved query %q: %w", query.ID, ErrNotFound)
	}
	return nil
}

// Delete removes a snippet, returning ErrNotFound if it does not exist.
func (r *SavedQueryRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM saved_queries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete saved query %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete saved query %q: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("saved query %q: %w", id, ErrNotFound)
	}
	return nil
}

func (r *SavedQueryRepository) query(ctx context.Context, statement string, args ...any) ([]model.SavedQuery, error) {
	rows, err := r.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list saved queries: %w", err)
	}
	defer rows.Close()

	queries := make([]model.SavedQuery, 0)
	for rows.Next() {
		query, err := scanSavedQuery(rows)
		if err != nil {
			return nil, fmt.Errorf("scan saved query: %w", err)
		}
		queries = append(queries, *query)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saved queries: %w", err)
	}
	return queries, nil
}

func scanSavedQuery(s scanner) (*model.SavedQuery, error) {
	var (
		query      model.SavedQuery
		connection sql.NullString
		database   sql.NullString
		tags       sql.NullString
	)
	if err := s.Scan(&query.ID, &connection, &database, &query.Title, &query.SQLText,
		&tags, &query.CreatedAt, &query.UpdatedAt); err != nil {
		return nil, err
	}
	query.ConnectionID = stringPtr(connection)
	query.DatabaseName = stringPtr(database)
	query.Tags = stringPtr(tags)
	return &query, nil
}
