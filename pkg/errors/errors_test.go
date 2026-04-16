package errors

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		msg     string
		wantNil bool
		wantMsg string
	}{
		{
			name:    "wraps error with context",
			err:     ErrNotFound,
			msg:     "taskRepo.GetByID",
			wantMsg: "taskRepo.GetByID: not found",
		},
		{
			name:    "returns nil for nil error",
			err:     nil,
			msg:     "some context",
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.msg)
			if tt.wantNil {
				assert.NoError(t, got)
				return
			}
			require.Error(t, got)
			assert.Equal(t, tt.wantMsg, got.Error())
		})
	}
}

func TestWrapf(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		format  string
		args    []any
		wantNil bool
		wantMsg string
	}{
		{
			name:    "wraps error with formatted context",
			err:     ErrNotFound,
			format:  "taskRepo.GetByID(%s)",
			args:    []any{"abc123"},
			wantMsg: "taskRepo.GetByID(abc123): not found",
		},
		{
			name:    "returns nil for nil error",
			err:     nil,
			format:  "context(%d)",
			args:    []any{42},
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrapf(tt.err, tt.format, tt.args...)
			if tt.wantNil {
				assert.NoError(t, got)
				return
			}
			require.Error(t, got)
			assert.Equal(t, tt.wantMsg, got.Error())
		})
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{
		ErrNotFound,
		ErrUnauthorized,
		ErrForbidden,
		ErrConflict,
		ErrAlreadyExists,
		ErrInvalidInput,
		ErrInternal,
		ErrUnavailable,
	}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				assert.True(t, stderrors.Is(a, b), "sentinel should match itself")
			} else {
				assert.False(t, stderrors.Is(a, b), "distinct sentinels should not match: %v vs %v", a, b)
			}
		}
	}
}

func TestWrappedErrors_PreserveSentinel(t *testing.T) {
	original := ErrNotFound
	wrapped := Wrap(original, "layer1")
	doubleWrapped := Wrapf(wrapped, "layer2(%s)", "ctx")

	assert.True(t, stderrors.Is(wrapped, ErrNotFound), "wrapped error should match sentinel")
	assert.True(t, stderrors.Is(doubleWrapped, ErrNotFound), "double-wrapped error should match sentinel")
	assert.False(t, stderrors.Is(doubleWrapped, ErrUnauthorized), "should not match different sentinel")
}

func TestWrap_preserves_sentinel_chain(t *testing.T) {
	err := fmt.Errorf("context: %w", ErrConflict)
	assert.True(t, stderrors.Is(err, ErrConflict))
	assert.False(t, stderrors.Is(err, ErrNotFound))
}
