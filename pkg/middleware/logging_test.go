package middleware

import (
	"context"
	"testing"

	"github.com/DeepakDP5/zee6do-server/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

func TestUnaryLogging_injects_logger_with_fields(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	baseLogger := zap.New(core)

	interceptor := UnaryLogging(baseLogger)

	// Inject a request ID first (as would happen in the real chain).
	ctx := context.WithValue(context.Background(), requestIDKey{}, "req-123")

	handler := func(ctx context.Context, req any) (any, error) {
		// The logger in context should have rpc_method and request_id.
		l := logging.FromContext(ctx)
		l.Info("from handler")
		return "ok", nil
	}

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Log"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	// Find the "from handler" log entry.
	handlerLogs := logs.FilterMessage("from handler")
	require.Equal(t, 1, handlerLogs.Len())

	entry := handlerLogs.All()[0]
	fieldMap := make(map[string]string)
	for _, f := range entry.Context {
		fieldMap[f.Key] = f.String
	}
	assert.Equal(t, "/test/Log", fieldMap["rpc_method"])
	assert.Equal(t, "req-123", fieldMap["request_id"])
}

func TestUnaryLogging_logs_completion(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	baseLogger := zap.New(core)

	interceptor := UnaryLogging(baseLogger)

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	_, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/test/Complete"}, handler)
	require.NoError(t, err)

	completionLogs := logs.FilterMessage("request completed")
	require.Equal(t, 1, completionLogs.Len())

	entry := completionLogs.All()[0]
	fieldMap := make(map[string]string)
	for _, f := range entry.Context {
		fieldMap[f.Key] = f.String
	}
	assert.Equal(t, "/test/Complete", fieldMap["rpc_method"])
	assert.Equal(t, "OK", fieldMap["grpc_code"])
}

func TestUnaryLogging_logs_error_completion(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	baseLogger := zap.New(core)

	interceptor := UnaryLogging(baseLogger)

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, assert.AnError
	}

	_, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/test/Error"}, handler)
	require.Error(t, err)

	warnLogs := logs.FilterMessage("request completed with error")
	require.Equal(t, 1, warnLogs.Len())
}

func TestUnaryLogging_completion_log_omits_downstream_user_id(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	baseLogger := zap.New(core)

	interceptor := UnaryLogging(baseLogger)

	// The auth interceptor (downstream) adds user_id to a child context,
	// but the logging interceptor holds its own ctx — so user_id is NOT
	// visible in the unary completion log. This is a known limitation.
	handler := func(ctx context.Context, req any) (any, error) {
		_ = withUserID(ctx, "user-xyz") // child context; caller never sees it
		return "ok", nil
	}

	_, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/test/UserID"}, handler)
	require.NoError(t, err)

	completionLogs := logs.FilterMessage("request completed")
	require.Equal(t, 1, completionLogs.Len())

	// Verify user_id is absent from the completion log fields.
	entry := completionLogs.All()[0]
	for _, f := range entry.Context {
		assert.NotEqual(t, "user_id", f.Key, "user_id should not be in unary completion log")
	}
}

func TestStreamLogging_injects_logger(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	baseLogger := zap.New(core)

	interceptor := StreamLogging(baseLogger)

	ctx := context.WithValue(context.Background(), requestIDKey{}, "stream-req-456")

	handler := func(srv any, stream grpc.ServerStream) error {
		l := logging.FromContext(stream.Context())
		l.Info("stream handler")
		return nil
	}

	ss := &fakeServerStream{ctx: ctx}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/StreamLog"}, handler)
	require.NoError(t, err)

	handlerLogs := logs.FilterMessage("stream handler")
	require.Equal(t, 1, handlerLogs.Len())

	entry := handlerLogs.All()[0]
	fieldMap := make(map[string]string)
	for _, f := range entry.Context {
		fieldMap[f.Key] = f.String
	}
	assert.Equal(t, "/test/StreamLog", fieldMap["rpc_method"])
	assert.Equal(t, "stream-req-456", fieldMap["request_id"])
}
