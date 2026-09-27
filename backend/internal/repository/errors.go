package repository

import "errors"

// Sentinel errors returned by repositories. Callers match them with errors.Is.
// Repository methods wrap these alongside a descriptive message and never
// return raw driver errors bare, so higher layers can map outcomes to stable
// API error codes (for example ErrNotFound -> 404 NOT_FOUND).
var (
	ErrNotFound = errors.New("repository: not found")
	ErrConflict = errors.New("repository: conflict")
)
