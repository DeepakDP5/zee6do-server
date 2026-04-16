package middleware

import (
	"context"
	"testing"

	"github.com/DeepakDP5/zee6do-server/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUnaryRecovery_no_panic(t *testing.T) {
	interceptor := UnaryRecovery()

	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryRecovery_catches_panic(t *testing.T) {
	interceptor := UnaryRecovery()

	// Inject a logger so recoverPanic doesn't panic on nil logger.
	ctx := logging.WithContext(context.Background(), zap.NewNop())

	handler := func(ctx context.Context, req any) (any, error) {
		panic("test panic")
	}

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Panic"}, handler)
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "internal error", st.Message())
}

func TestUnaryRecovery_catches_non_string_panic(t *testing.T) {
	interceptor := UnaryRecovery()
	ctx := logging.WithContext(context.Background(), zap.NewNop())

	handler := func(ctx context.Context, req any) (any, error) {
		panic(42) // non-string panic value
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Panic"}, handler)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestStreamRecovery_catches_panic(t *testing.T) {
	interceptor := StreamRecovery()
	ctx := logging.WithContext(context.Background(), zap.NewNop())

	handler := func(srv any, stream grpc.ServerStream) error {
		panic("stream panic")
	}

	ss := &fakeServerStream{ctx: ctx}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/StreamPanic"}, handler)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestStreamRecovery_no_panic(t *testing.T) {
	interceptor := StreamRecovery()

	handler := func(srv any, stream grpc.ServerStream) error {
		return nil
	}

	ss := &fakeServerStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/Stream"}, handler)
	require.NoError(t, err)
}
