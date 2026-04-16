package middleware

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeJWTValidator implements JWTValidator for tests.
type fakeJWTValidator struct {
	userID string
	err    error
}

func (f *fakeJWTValidator) ValidateToken(_ context.Context, token string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.userID, nil
}

func newAuthConfig(validator JWTValidator, skipMethods ...string) AuthConfig {
	skip := make(map[string]bool)
	for _, m := range skipMethods {
		skip[m] = true
	}
	return AuthConfig{Validator: validator, SkipMethods: skip}
}

func TestUserIDFromContext_empty(t *testing.T) {
	assert.Equal(t, "", UserIDFromContext(context.Background()))
}

func TestUserIDFromContext_present(t *testing.T) {
	ctx := withUserID(context.Background(), "user-abc")
	assert.Equal(t, "user-abc", UserIDFromContext(ctx))
}

func TestUnaryAuth_skip_method(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{}, "/test/SkippedMethod")
	interceptor := UnaryAuth(cfg)

	var handlerCalled bool
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		// User ID should NOT be set for skipped methods.
		assert.Equal(t, "", UserIDFromContext(ctx))
		return "ok", nil
	}

	// No metadata at all — should still pass because method is skipped.
	resp, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/test/SkippedMethod"}, handler)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
	assert.Equal(t, "ok", resp)
}

func TestUnaryAuth_missing_metadata(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{userID: "user1"})
	interceptor := UnaryAuth(cfg)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	_, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Equal(t, "missing metadata", st.Message())
}

func TestUnaryAuth_missing_authorization_header(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{userID: "user1"})
	interceptor := UnaryAuth(cfg)

	// Metadata present but no authorization header.
	md := metadata.Pairs("other-header", "value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Equal(t, "missing authorization header", st.Message())
}

func TestUnaryAuth_empty_authorization_header(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{userID: "user1"})
	interceptor := UnaryAuth(cfg)

	md := metadata.Pairs("authorization", "")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestUnaryAuth_bearer_prefix_stripped(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{userID: "user-from-token"})
	interceptor := UnaryAuth(cfg)

	md := metadata.Pairs("authorization", "Bearer my-jwt-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var capturedCtx context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		capturedCtx = ctx
		return "ok", nil
	}

	resp, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Equal(t, "user-from-token", UserIDFromContext(capturedCtx))
}

func TestUnaryAuth_token_without_bearer_prefix(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{userID: "user-raw"})
	interceptor := UnaryAuth(cfg)

	md := metadata.Pairs("authorization", "raw-token-value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var capturedCtx context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		capturedCtx = ctx
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "user-raw", UserIDFromContext(capturedCtx))
}

func TestUnaryAuth_invalid_token(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{err: fmt.Errorf("token expired")})
	interceptor := UnaryAuth(cfg)

	md := metadata.Pairs("authorization", "Bearer expired-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Equal(t, "invalid token", st.Message())
}

func TestUnaryAuth_bearer_only_empty_token(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{userID: "user1"})
	interceptor := UnaryAuth(cfg)

	// "Bearer " with nothing after it.
	md := metadata.Pairs("authorization", "Bearer ")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, handler)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Equal(t, "empty token", st.Message())
}

func TestStreamAuth_skip_method(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{}, "/test/SkippedStream")
	interceptor := StreamAuth(cfg)

	var handlerCalled bool
	handler := func(srv any, stream grpc.ServerStream) error {
		handlerCalled = true
		return nil
	}

	ss := &fakeServerStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/SkippedStream"}, handler)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestStreamAuth_valid_token(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{userID: "stream-user"})
	interceptor := StreamAuth(cfg)

	md := metadata.Pairs("authorization", "Bearer stream-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	var capturedCtx context.Context
	handler := func(srv any, stream grpc.ServerStream) error {
		capturedCtx = stream.Context()
		return nil
	}

	ss := &fakeServerStream{ctx: ctx}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/Stream"}, handler)
	require.NoError(t, err)
	assert.Equal(t, "stream-user", UserIDFromContext(capturedCtx))
}

func TestStreamAuth_missing_metadata(t *testing.T) {
	cfg := newAuthConfig(&fakeJWTValidator{userID: "user1"})
	interceptor := StreamAuth(cfg)

	handler := func(srv any, stream grpc.ServerStream) error {
		t.Fatal("handler should not be called")
		return nil
	}

	ss := &fakeServerStream{ctx: context.Background()}
	err := interceptor(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test/Stream"}, handler)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}
