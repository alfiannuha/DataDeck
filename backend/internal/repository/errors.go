package repository

import "errors"

// Sentinel errors returned by repositories. Callers match them with errors.Is.
// Repository methods wrap these alongside a descriptive message and never
// return raw driver errors bare, so higher layers can map outcomes to stable
// API error codes (for example ErrNotFound -> 404 NOT_FOUND).
var (
	ErrNotFound = errors.New("repository: not found")
	ErrConflict = errors.New("repository: conflict")
	// ErrInvalidOrder marks a connection reorder request that is not a valid
	// permutation of all existing connections (unknown/duplicate/missing ids).
	ErrInvalidOrder = errors.New("repository: invalid connection order")
)
