package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net"
	"syscall"

	"github.com/datadeck/datadeck/backend/internal/model"
)

// SQLError is the driver-neutral representation of a statement error returned
// by a target database. Handlers map it to API error codes without importing
// any driver package.
type SQLError struct {
	Driver model.Driver
	// Code is the driver's native error code when available (PostgreSQL
	// SQLSTATE, MySQL error number as a string).
	Code string
	// Message is safe to show to the user (no credentials/DSN).
	Message string
	// Position is the 1-based character position when the driver reports one.
	Position int
	// Syntax marks errors the driver classifies as SQL syntax errors, so the
	// handler can choose SQL_SYNTAX_ERROR without knowing driver codes.
	Syntax bool
}

func (e *SQLError) Error() string { return e.Message }

// isConnectionLoss reports whether err represents a lost/broken target
// connection rather than a statement error. Only transport-level failures are
// matched (bad conn, closed conn, network errors, EOF/reset/refused), so SQL
// execution errors are never reclassified. Context errors are excluded so
// timeouts/cancellation keep their own handling.
func isConnectionLoss(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, sql.ErrConnDone) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNABORTED) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// asSQLError returns the neutral SQLError when err carries one.
func asSQLError(err error) (*SQLError, bool) {
	var sqlErr *SQLError
	if errors.As(err, &sqlErr) {
		return sqlErr, true
	}
	return nil, false
}
