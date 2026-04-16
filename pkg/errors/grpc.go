// Package errors provides gRPC status code mapping for sentinel errors.
// This file maps domain errors to the appropriate gRPC status codes
// at handler boundaries.
package errors

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToGRPCStatus converts a domain error to a gRPC *status.Status.
// It checks against known sentinel errors and maps to the appropriate code.
// Unknown errors map to codes.Internal with a generic message to avoid leaking internals.
func ToGRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}

	switch {
	case errors.Is(err, ErrNotFound):
		return status.New(codes.NotFound, "not found")
	case errors.Is(err, ErrUnauthorized):
		return status.New(codes.Unauthenticated, "unauthenticated")
	case errors.Is(err, ErrForbidden):
		return status.New(codes.PermissionDenied, "permission denied")
	case errors.Is(err, ErrAlreadyExists):
		return status.New(codes.AlreadyExists, "already exists")
	case errors.Is(err, ErrConflict):
		return status.New(codes.AlreadyExists, "conflict")
	case errors.Is(err, ErrInvalidInput):
		return status.New(codes.InvalidArgument, "invalid argument")
	case errors.Is(err, ErrRateLimited):
		return status.New(codes.ResourceExhausted, "rate limited")
	case errors.Is(err, ErrSubscriptionExpired):
		return status.New(codes.FailedPrecondition, "subscription expired")
	case errors.Is(err, ErrUnavailable):
		return status.New(codes.Unavailable, "service unavailable")
	case errors.Is(err, ErrInternal):
		return status.New(codes.Internal, "internal error")
	default:
		return status.New(codes.Internal, "internal error")
	}
}

// ToGRPCError converts a domain error to a gRPC error suitable for returning
// from a gRPC handler. Convenience wrapper around ToGRPCStatus.
func ToGRPCError(err error) error {
	if err == nil {
		return nil
	}
	return ToGRPCStatus(err).Err()
}
