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
	ID                   string
	Name                 string
	Driver               Driver
	Host                 *string
	Port                 *int
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
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Redacted returns a copy of the profile with credential ciphertext removed.
// API responses must use this (or an equivalent secret-free DTO): even
// ciphertext is never returned to clients.
func (p ConnectionProfile) Redacted() ConnectionProfile {
	p.EncryptedPassword = nil
	p.SSHEncryptedPassword = nil
	return p
}
