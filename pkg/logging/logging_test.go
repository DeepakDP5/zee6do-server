package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{name: "creates info logger", level: "info"},
		{name: "creates debug logger", level: "debug"},
		{name: "creates warn logger", level: "warn"},
		{name: "creates error logger", level: "error"},
		{name: "defaults to info for invalid level", level: "invalid"},
		{name: "defaults to info for empty level", level: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.level)
			require.NotNil(t, logger)
			// Should not panic when used
			logger.Info("test message")
		})
	}
}

func TestNewDevelopmentLogger(t *testing.T) {
	logger := NewDevelopmentLogger()
	require.NotNil(t, logger)
	logger.Info("dev test message")
}

func TestWithContext_and_FromContext(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	ctx := WithContext(context.Background(), logger)
	retrieved := FromContext(ctx)

	retrieved.Info("test message")
	assert.Equal(t, 1, recorded.Len(), "should have recorded one log entry")
	assert.Equal(t, "test message", recorded.All()[0].Message)
}

func TestFromContext_returns_nop_when_no_logger(t *testing.T) {
	logger := FromContext(context.Background())
	require.NotNil(t, logger, "should return a non-nil nop logger")
	// Should not panic when used
	logger.Info("this goes nowhere")
}

func TestWithFields(t *testing.T) {
	core, recorded := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	ctx := WithContext(context.Background(), logger)
	ctx = WithFields(ctx,
		zap.String("request_id", "req-123"),
		zap.String("user_id", "user-456"),
	)

	FromContext(ctx).Info("request processed")

	require.Equal(t, 1, recorded.Len())
	entry := recorded.All()[0]
	assert.Equal(t, "request processed", entry.Message)

	fields := entry.ContextMap()
	assert.Equal(t, "req-123", fields["request_id"])
	assert.Equal(t, "user-456", fields["user_id"])
}

func TestWithFields_on_context_without_logger(t *testing.T) {
	// Should not panic, uses nop logger
	ctx := WithFields(context.Background(), zap.String("key", "value"))
	logger := FromContext(ctx)
	require.NotNil(t, logger)
}
