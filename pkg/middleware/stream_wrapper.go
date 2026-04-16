package middleware

import (
	"context"

	"google.golang.org/grpc"
)

// wrappedServerStream wraps grpc.ServerStream to override Context().
// It is shared across interceptors (request_id, auth, logging) that need
// to propagate a modified context through the stream handler chain.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
