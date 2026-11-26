package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// ConnectionGroupRepository persists optional connection groups in the embedded
// store. Deleting a group relies on the schema's ON DELETE SET NULL so member
// connections survive and become ungrouped.
type ConnectionGroupRepository struct {
	db *sql.DB
}

func NewConnectionGroupRepository(db *sql.DB) *ConnectionGroupRepository {
	return &ConnectionGroupRepository{db: db}
}

const groupColumns = `id, name, sort_order, is_expanded, created_at, updated_at`

// Create appends a new group at the bottom (collapsed by default).
func (r *ConnectionGroupRepository) Create(ctx context.Context, group *model.ConnectionGroup) error {
	if group == nil {
		return errors.New("group repository: group must not be nil")
	}
	now := time.Now().UTC()
	if group.CreatedAt.IsZero() {
		group.CreatedAt = now
	}
	group.UpdatedAt = group.CreatedAt

	var next int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(sort_order)+1, 0) FROM connection_groups`).Scan(&next); err != nil {
		return fmt.Errorf("next group order: %w", err)
	}
	group.SortOrder = next

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO connection_groups (`+groupColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		group.ID, group.Name, group.SortOrder, boolToInt(group.IsExpanded),
		group.CreatedAt, group.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("group %q: %w", group.Name, ErrConflict)
		}
		return fmt.Errorf("create group %q: %w", group.ID, err)
	}
	return nil
}

// List returns groups ordered by manual position (deterministic name/id tie-break).
func (r *ConnectionGroupRepository) List(ctx context.Context) ([]model.ConnectionGroup, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+groupColumns+` FROM connection_groups ORDER BY sort_order ASC, name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	groups := make([]model.ConnectionGroup, 0)
	for rows.Next() {
		var (
			group      model.ConnectionGroup
			isExpanded int
		)
		if err := rows.Scan(&group.ID, &group.Name, &group.SortOrder, &isExpanded, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		group.IsExpanded = isExpanded != 0
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}
	return groups, nil
}

// Get returns one group or ErrNotFound.
func (r *ConnectionGroupRepository) Get(ctx context.Context, id string) (model.ConnectionGroup, error) {
	var (
		group      model.ConnectionGroup
		isExpanded int
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT `+groupColumns+` FROM connection_groups WHERE id = ?`, id).
		Scan(&group.ID, &group.Name, &group.SortOrder, &isExpanded, &group.CreatedAt, &group.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ConnectionGroup{}, fmt.Errorf("group %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return model.ConnectionGroup{}, fmt.Errorf("get group %q: %w", id, err)
	}
	group.IsExpanded = isExpanded != 0
	return group, nil
}

// Update renames a group and/or updates its expanded state. Other fields are kept.
func (r *ConnectionGroupRepository) Update(ctx context.Context, group *model.ConnectionGroup) error {
	group.UpdatedAt = time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`UPDATE connection_groups SET name = ?, is_expanded = ?, updated_at = ? WHERE id = ?`,
		group.Name, boolToInt(group.IsExpanded), group.UpdatedAt, group.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("group %q: %w", group.Name, ErrConflict)
		}
		return fmt.Errorf("update group %q: %w", group.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update group %q: %w", group.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("group %q: %w", group.ID, ErrNotFound)
	}
	return nil
}

// Delete removes a group; member connections become ungrouped (ON DELETE SET NULL).
func (r *ConnectionGroupRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM connection_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete group %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete group %q: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("group %q: %w", id, ErrNotFound)
	}
	return nil
}

// Reorder persists a manual group order (ids must be a permutation of all groups).
func (r *ConnectionGroupRepository) Reorder(ctx context.Context, ids []string) error {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM connection_groups`)
	if err != nil {
		return fmt.Errorf("list group ids: %w", err)
	}
	existing := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan group id: %w", err)
		}
		existing[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate group ids: %w", err)
	}
	_ = rows.Close()

	if len(ids) != len(existing) {
		return fmt.Errorf("%w: expected %d ids, got %d", ErrInvalidOrder, len(existing), len(ids))
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			return fmt.Errorf("%w: unknown group %q", ErrInvalidOrder, id)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("%w: duplicate group %q", ErrInvalidOrder, id)
		}
		seen[id] = struct{}{}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin group reorder: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for position, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE connection_groups SET sort_order = ? WHERE id = ?`, position, id); err != nil {
			return fmt.Errorf("reorder group %q: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit group reorder: %w", err)
	}
	return nil
}
