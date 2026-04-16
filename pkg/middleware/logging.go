package middleware

import (
	"context"
	"time"

	"github.com/DeepakDP5/zee6do-server/pkg/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryLogging returns a unary server interceptor that injects a
// request-scoped Zap logger into the context and logs request summaries.
// The logger carries request_id, rpc_method, and (if authenticated) user_id.
func UnaryLogging(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx = enrichContext(ctx, logger, info.FullMethod)
		start := time.Now()

		resp, err := handler(ctx, req)

		logRequestSummary(ctx, info.FullMethod, start, err)
		return resp, err
	}
}

// StreamLogging returns a stream server interceptor that injects a
// request-scoped Zap logger into the context and logs request summaries.
func StreamLogging(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := enrichContext(ss.Context(), logger, info.FullMethod)
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}
		start := time.Now()

		err := handler(srv, wrapped)

		// Note: For stream RPCs, user_id from the auth interceptor is stored
		// in a deeper wrappedServerStream created downstream. This wrapper's
		// context does not have user_id since each interceptor creates its own
		// wrappedServerStream. The unary interceptor does not have this
		// limitation because context flows back through the handler chain.
		// This is a known trade-off — stream completion logs omit user_id.
		logRequestSummary(wrapped.Context(), info.FullMethod, start, err)
		return err
	}
}

// enrichContext adds request-scoped fields to the logger and injects it into context.
// Note: user_id is not available here because the auth interceptor runs after logging.
// The completion log in logRequestSummary re-fetches user_id from context after auth has run.
func enrichContext(ctx context.Context, baseLogger *zap.Logger, method string) context.Context {
	fields := []zap.Field{
		zap.String("rpc_method", method),
	}
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	return logging.WithContext(ctx, baseLogger.With(fields...))
}

// logRequestSummary logs the outcome of a completed request at Info level.
// It attempts to read user_id from the provided context. Note: for both unary
// and stream RPCs, user_id is typically NOT present because the auth interceptor
// stores it in a child context that this interceptor does not hold. This is a
// known limitation — the completion log omits user_id.
func logRequestSummary(ctx context.Context, method string, start time.Time, err error) {
	logger := logging.FromContext(ctx)
	duration := time.Since(start)

	fields := []zap.Field{
		zap.String("rpc_method", method),
		zap.Duration("duration", duration),
	}

	// Add user_id if auth interceptor populated it downstream.
	if userID := UserIDFromContext(ctx); userID != "" {
		fields = append(fields, zap.String("user_id", userID))
	}

	if err != nil {
		st, _ := status.FromError(err)
		fields = append(fields, zap.String("grpc_code", st.Code().String()))
		logger.Warn("request completed with error", fields...)
	} else {
		fields = append(fields, zap.String("grpc_code", "OK"))
		logger.Info("request completed", fields...)
	}
}
