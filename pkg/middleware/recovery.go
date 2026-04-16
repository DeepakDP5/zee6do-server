package middleware

import (
	"context"
	"runtime/debug"

	"github.com/DeepakDP5/zee6do-server/pkg/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryRecovery returns a unary server interceptor that recovers from panics
// in downstream handlers. Panics are logged with a stack trace and converted
// to a codes.Internal gRPC error so they don't crash the server.
func UnaryRecovery() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer recoverPanic(ctx, &err)
		return handler(ctx, req)
	}
}

// StreamRecovery returns a stream server interceptor that recovers from panics.
func StreamRecovery() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer recoverPanic(ss.Context(), &err)
		return handler(srv, ss)
	}
}

func recoverPanic(ctx context.Context, errPtr *error) { //nolint:gocritic // *error is required for defer/recover to set the named return value
	if r := recover(); r != nil {
		logger := logging.FromContext(ctx)
		stack := string(debug.Stack())
		logger.Error("panic recovered in gRPC handler",
			zap.Any("panic", r),
			zap.String("stack", stack),
		)
		*errPtr = status.Errorf(codes.Internal, "internal error")
	}
}
