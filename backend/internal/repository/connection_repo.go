package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// ConnectionRepository persists connection profiles in the embedded store.
// It stores whatever encrypted credential value the security layer supplies and
// never encrypts, decrypts, or inspects credentials.
type ConnectionRepository struct {
	db *sql.DB
}

// NewConnectionRepository returns a repository backed by db.
func NewConnectionRepository(db *sql.DB) *ConnectionRepository {
	return &ConnectionRepository{db: db}
}

const connectionColumns = `id, name, driver, host, port, database_name, username,
	encrypted_password, ssl_mode, ssh_enabled, ssh_host, ssh_port, ssh_username,
	ssh_encrypted_password, ssh_key_path, created_at, updated_at`

// Create inserts a new connection profile, assigning timestamps when unset.
func (r *ConnectionRepository) Create(ctx context.Context, profile *model.ConnectionProfile) error {
	if profile == nil {
		return errors.New("connection repository: profile must not be nil")
	}
	now := time.Now().UTC()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	if profile.UpdatedAt.IsZero() {
		profile.UpdatedAt = profile.CreatedAt
	}
	sslMode := profile.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	profile.SSLMode = sslMode

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO connection_profiles (`+connectionColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profile.ID, profile.Name, string(profile.Driver), nullString(profile.Host), nullInt(profile.Port),
		profile.DatabaseName, nullString(profile.Username), nullString(profile.EncryptedPassword), sslMode,
		boolToInt(profile.SSHEnabled), nullString(profile.SSHHost), nullInt(profile.SSHPort),
		nullString(profile.SSHUsername), nullString(profile.SSHEncryptedPassword), nullString(profile.SSHKeyPath),
		profile.CreatedAt, profile.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("connection %q: %w", profile.ID, ErrConflict)
		}
		return fmt.Errorf("create connection %q: %w", profile.ID, err)
	}
	return nil
}

// Get returns the profile with the given id, or ErrNotFound.
func (r *ConnectionRepository) Get(ctx context.Context, id string) (*model.ConnectionProfile, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+connectionColumns+` FROM connection_profiles WHERE id = ?`, id)
	profile, err := scanConnection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("connection %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get connection %q: %w", id, err)
	}
	return profile, nil
}

// List returns all profiles ordered by name then id for deterministic output.
func (r *ConnectionRepository) List(ctx context.Context) ([]model.ConnectionProfile, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+connectionColumns+` FROM connection_profiles ORDER BY name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()

	profiles := make([]model.ConnectionProfile, 0)
	for rows.Next() {
		profile, err := scanConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("scan connection: %w", err)
		}
		profiles = append(profiles, *profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connections: %w", err)
	}
	return profiles, nil
}

// Update replaces the mutable fields of an existing profile and refreshes
// UpdatedAt. It returns ErrNotFound if no row matches.
func (r *ConnectionRepository) Update(ctx context.Context, profile *model.ConnectionProfile) error {
	if profile == nil {
		return errors.New("connection repository: profile must not be nil")
	}
	profile.UpdatedAt = time.Now().UTC()
	sslMode := profile.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	profile.SSLMode = sslMode

	res, err := r.db.ExecContext(ctx,
		`UPDATE connection_profiles SET
			name = ?, driver = ?, host = ?, port = ?, database_name = ?, username = ?,
			encrypted_password = ?, ssl_mode = ?, ssh_enabled = ?, ssh_host = ?, ssh_port = ?,
			ssh_username = ?, ssh_encrypted_password = ?, ssh_key_path = ?, updated_at = ?
		 WHERE id = ?`,
		profile.Name, string(profile.Driver), nullString(profile.Host), nullInt(profile.Port),
		profile.DatabaseName, nullString(profile.Username), nullString(profile.EncryptedPassword), sslMode,
		boolToInt(profile.SSHEnabled), nullString(profile.SSHHost), nullInt(profile.SSHPort),
		nullString(profile.SSHUsername), nullString(profile.SSHEncryptedPassword), nullString(profile.SSHKeyPath),
		profile.UpdatedAt, profile.ID)
	if err != nil {
		return fmt.Errorf("update connection %q: %w", profile.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update connection %q: %w", profile.ID, err)
	}
	if n == 0 {
		return fmt.Errorf("connection %q: %w", profile.ID, ErrNotFound)
	}
	return nil
}

// Delete removes a profile, returning ErrNotFound if it does not exist. Related
// history rows cascade and saved-query references are nulled by the schema.
func (r *ConnectionRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM connection_profiles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete connection %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete connection %q: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("connection %q: %w", id, ErrNotFound)
	}
	return nil
}

func scanConnection(s scanner) (*model.ConnectionProfile, error) {
	var (
		profile    model.ConnectionProfile
		driver     string
		host       sql.NullString
		port       sql.NullInt64
		username   sql.NullString
		encrypted  sql.NullString
		sslMode    sql.NullString
		sshEnabled int
		sshHost    sql.NullString
		sshPort    sql.NullInt64
		sshUser    sql.NullString
		sshEncPw   sql.NullString
		sshKey     sql.NullString
	)
	if err := s.Scan(
		&profile.ID, &profile.Name, &driver, &host, &port, &profile.DatabaseName,
		&username, &encrypted, &sslMode, &sshEnabled, &sshHost, &sshPort,
		&sshUser, &sshEncPw, &sshKey, &profile.CreatedAt, &profile.UpdatedAt,
	); err != nil {
		return nil, err
	}

	profile.Driver = model.Driver(driver)
	profile.Host = stringPtr(host)
	profile.Port = intPtr(port)
	profile.Username = stringPtr(username)
	profile.EncryptedPassword = stringPtr(encrypted)
	profile.SSLMode = sslMode.String
	if profile.SSLMode == "" {
		profile.SSLMode = "disable"
	}
	profile.SSHEnabled = sshEnabled != 0
	profile.SSHHost = stringPtr(sshHost)
	profile.SSHPort = intPtr(sshPort)
	profile.SSHUsername = stringPtr(sshUser)
	profile.SSHEncryptedPassword = stringPtr(sshEncPw)
	profile.SSHKeyPath = stringPtr(sshKey)
	return &profile, nil
}
