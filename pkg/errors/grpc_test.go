package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
)

func TestToGRPCStatus_nil_error(t *testing.T) {
	st := ToGRPCStatus(nil)
	assert.Equal(t, codes.OK, st.Code())
}

func TestToGRPCStatus_sentinel_mapping(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode codes.Code
		expectedMsg  string
	}{
		{"NotFound", ErrNotFound, codes.NotFound, "not found"},
		{"Unauthorized", ErrUnauthorized, codes.Unauthenticated, "unauthenticated"},
		{"Forbidden", ErrForbidden, codes.PermissionDenied, "permission denied"},
		{"AlreadyExists", ErrAlreadyExists, codes.AlreadyExists, "already exists"},
		{"Conflict", ErrConflict, codes.AlreadyExists, "conflict"},
		{"InvalidInput", ErrInvalidInput, codes.InvalidArgument, "invalid argument"},
		{"RateLimited", ErrRateLimited, codes.ResourceExhausted, "rate limited"},
		{"SubscriptionExpired", ErrSubscriptionExpired, codes.FailedPrecondition, "subscription expired"},
		{"Unavailable", ErrUnavailable, codes.Unavailable, "service unavailable"},
		{"Internal", ErrInternal, codes.Internal, "internal error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := ToGRPCStatus(tc.err)
			assert.Equal(t, tc.expectedCode, st.Code())
			assert.Equal(t, tc.expectedMsg, st.Message())
		})
	}
}

func TestToGRPCStatus_wrapped_errors(t *testing.T) {
	wrapped := fmt.Errorf("repo.GetByID: %w", ErrNotFound)
	st := ToGRPCStatus(wrapped)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestToGRPCStatus_unknown_error(t *testing.T) {
	unknown := fmt.Errorf("something unexpected")
	st := ToGRPCStatus(unknown)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "internal error", st.Message())
}

func TestToGRPCError_nil(t *testing.T) {
	assert.Nil(t, ToGRPCError(nil))
}

func TestToGRPCError_returns_error(t *testing.T) {
	err := ToGRPCError(ErrNotFound)
	assert.Error(t, err)
}
