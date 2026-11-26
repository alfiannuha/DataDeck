package model

import "time"

// Driver identifies a supported target database driver.
type Driver string

const (
	DriverPostgres Driver = "postgres"
	DriverMySQL    Driver = "mysql"
	DriverSQLite   Driver = "sqlite"
)

// Valid reports whether d is a driver the application store accepts.
func (d Driver) Valid() bool {
	switch d {
	case DriverPostgres, DriverMySQL, DriverSQLite:
		return true
	default:
		return false
	}
}

// ConnectionProfile is a persisted connection profile.
//
// EncryptedPassword and SSHEncryptedPassword always hold ciphertext produced by
// the security layer; this type never carries plaintext credentials.
//
// Nullable storage columns (host, port, username, passwords, SSH fields) are
// represented as pointers so the difference between "absent" and "empty" is
// preserved.
type ConnectionProfile struct {
	ID     string
	Name   string
	Driver Driver
	Host   *string
	Port   *int
	// DatabaseName is the target database. For PostgreSQL (PRF-01) it is an
	// optional default database: a profile may represent a server/instance and
	// the empty string means "no default selected yet". MySQL and SQLite keep
	// requiring a value (database / file path).
	DatabaseName         string
	Username             *string
	EncryptedPassword    *string
	SSLMode              string
	SSHEnabled           bool
	SSHHost              *string
	SSHPort              *int
	SSHUsername          *string
	SSHEncryptedPassword *string
	SSHKeyPath           *string
	// Color is a presentation hint (a #RRGGBB hex string) used to distinguish
	// connections in the UI. The empty string means "no color assigned"; the API
	// substitutes a default for display. It never carries credentials.
	Color string
	// SortOrder is the manual sidebar position (0-based, unique). New
	// connections are appended at the bottom; editing never changes it.
	SortOrder int
	// GroupID is the optional connection group this profile belongs to. nil means
	// ungrouped (rendered directly in the main list).
	GroupID   *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ConnectionGroup is an optional, user-defined grouping of connections shown as
// an accordion in the sidebar. Membership is explicit; connections without a
// group remain ungrouped.
type ConnectionGroup struct {
	ID         string
	Name       string
	SortOrder  int
	IsExpanded bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Redacted returns a copy of the profile with credential ciphertext removed.
// API responses must use this (or an equivalent secret-free DTO): even
// ciphertext is never returned to clients.
func (p ConnectionProfile) Redacted() ConnectionProfile {
	p.EncryptedPassword = nil
	p.SSHEncryptedPassword = nil
	return p
}
