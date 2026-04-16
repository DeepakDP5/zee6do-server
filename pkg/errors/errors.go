// Package errors provides sentinel errors and wrapping utilities for the zee6do server.
// All errors should be wrapped with context using fmt.Errorf("context: %w", err) and
// checked using errors.Is / errors.As at handler boundaries.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for known, checkable conditions.
// gRPC status mapping happens at the handler boundary only.
var (
	ErrNotFound      = errors.New("not found")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
	ErrConflict      = errors.New("conflict")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInternal      = errors.New("internal error")
	ErrUnavailable   = errors.New("service unavailable")
)

// Wrap adds context to an error using fmt.Errorf with %w.
// Returns nil if err is nil.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf adds formatted context to an error using fmt.Errorf with %w.
// Returns nil if err is nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// Is reports whether any error in err's tree matches target.
// Re-exported from the standard library for convenience.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's tree that matches target,
// and if so, sets target to that error value and returns true.
// Re-exported from the standard library for convenience.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// New creates a new error with the given text.
// Re-exported from the standard library for convenience.
func New(text string) error {
	return errors.New(text)
}
