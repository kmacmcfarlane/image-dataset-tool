package service

import "errors"

// Sentinel errors for service operations.
var (
	ErrBadRequest = errors.New("bad request")
	ErrConflict   = errors.New("conflict")
)
