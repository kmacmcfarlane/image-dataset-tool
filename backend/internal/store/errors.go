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
	ErrDuplicateLink    = errors.New("duplicate link")
)

// isDuplicateConstraint checks if a SQLite error indicates a UNIQUE constraint violation.
func isDuplicateConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// isForeignKeyConstraint checks if a SQLite error indicates a FOREIGN KEY constraint violation.
func isForeignKeyConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}
