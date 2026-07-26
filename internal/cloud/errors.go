package cloud

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrQuotaExceeded = errors.New("quota exceeded")
	ErrConflict      = errors.New("conflict")
	ErrRateLimited   = errors.New("rate limited")
	ErrUnavailable   = errors.New("unavailable")
	ErrOutOfRange    = errors.New("out of range")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrInvalid       = errors.New("invalid request")
)
