package store

import (
	"errors"
	"strings"
)

// Sentinel errors for store operations.
var (
	ErrNotFound         = errors.New("not found")
	ErrDuplicateSlug    = errors.New("duplicate slug")
	ErrDuplicateAccount = errors.New("duplicate account")
)

// isDuplicateConstraint checks if a SQLite error indicates a UNIQUE constraint violation.
func isDuplicateConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
