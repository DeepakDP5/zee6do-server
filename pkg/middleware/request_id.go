// Package middleware provides gRPC interceptors for the zee6do server.
// Interceptors are chained in order: request_id → logging → recovery → auth → validation.
package middleware

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type requestIDKey struct{}

const (
	// RequestIDHeader is the metadata key used to carry the request ID.
	RequestIDHeader = "x-request-id"
)

// RequestIDFromContext extracts the request ID from the context.
// Returns an empty string if no request ID is set.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// UnaryRequestID returns a unary server interceptor that injects a unique
// request ID into the context. If the incoming metadata already has an
// x-request-id, it is reused; otherwise a new UUID is generated.
func UnaryRequestID() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = injectRequestID(ctx)
		return handler(ctx, req)
	}
}

// StreamRequestID returns a stream server interceptor that injects a unique
// request ID into the context.
func StreamRequestID() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := injectRequestID(ss.Context())
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
	}
}

func injectRequestID(ctx context.Context) context.Context {
	// Check incoming metadata for an existing request ID.
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(RequestIDHeader); len(vals) > 0 && vals[0] != "" {
			return context.WithValue(ctx, requestIDKey{}, vals[0])
		}
	}
	return context.WithValue(ctx, requestIDKey{}, uuid.New().String())
}

// wrappedServerStream wraps grpc.ServerStream to override Context().
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
