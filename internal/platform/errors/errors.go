package apperrors

import "errors"

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrNotFound            = errors.New("not found")
	ErrNoActiveSession     = errors.New("no active session")
	ErrActiveSessionExists = errors.New("active session already exists")
)
