package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestIDFromContext_empty(t *testing.T) {
	assert.Equal(t, "", RequestIDFromContext(context.Background()))
}

func TestRequestIDFromContext_present(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey{}, "test-id-123")
	assert.Equal(t, "test-id-123", RequestIDFromContext(ctx))
}

func TestUnaryRequestID_generates_uuid(t *testing.T) {
	interceptor := UnaryRequestID()

	var capturedCtx context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)

	id := RequestIDFromContext(capturedCtx)
	assert.NotEmpty(t, id)
	// UUID format: 8-4-4-4-12
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
}

func TestUnaryRequestID_reuses_existing_id(t *testing.T) {
	interceptor := UnaryRequestID()

	// Set incoming metadata with an existing request ID.
	md := metadata.Pairs(RequestIDHeader, "existing-req-id")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var capturedCtx context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		capturedCtx = ctx
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	require.NoError(t, err)

	assert.Equal(t, "existing-req-id", RequestIDFromContext(capturedCtx))
}

func TestUnaryRequestID_ignores_empty_existing_id(t *testing.T) {
	interceptor := UnaryRequestID()

	md := metadata.Pairs(RequestIDHeader, "")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var capturedCtx context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		capturedCtx = ctx
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	require.NoError(t, err)

	id := RequestIDFromContext(capturedCtx)
	assert.NotEmpty(t, id) // Should generate a new UUID, not reuse empty string
}

func TestStreamRequestID_generates_uuid(t *testing.T) {
	interceptor := StreamRequestID()

	var capturedCtx context.Context
	handler := func(srv any, stream grpc.ServerStream) error {
		capturedCtx = stream.Context()
		return nil
	}

	ss := &fakeServerStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/Stream"}, handler)
	require.NoError(t, err)

	id := RequestIDFromContext(capturedCtx)
	assert.NotEmpty(t, id)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
}

func TestStreamRequestID_reuses_existing_id(t *testing.T) {
	interceptor := StreamRequestID()

	md := metadata.Pairs(RequestIDHeader, "stream-req-id")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var capturedCtx context.Context
	handler := func(srv any, stream grpc.ServerStream) error {
		capturedCtx = stream.Context()
		return nil
	}

	ss := &fakeServerStream{ctx: ctx}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/Stream"}, handler)
	require.NoError(t, err)

	assert.Equal(t, "stream-req-id", RequestIDFromContext(capturedCtx))
}

// fakeServerStream is a minimal grpc.ServerStream implementation for tests.
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context {
	return f.ctx
}
