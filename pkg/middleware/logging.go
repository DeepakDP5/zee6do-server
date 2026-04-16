package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/DeepakDP5/zee6do-server/pkg/logging"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// logFieldsKey is the context key for the shared request-scoped log fields holder.
type logFieldsKey struct{}

// logFields is a mutable holder for request-scoped log fields that downstream
// interceptors (e.g. auth) can append to. The logging interceptor reads these
// fields when producing the completion log, ensuring user_id and other
// post-auth fields appear without the limitations of immutable context propagation.
type logFields struct {
	mu     sync.Mutex
	fields []zap.Field
}

// AddField appends a field to the shared log-fields holder. Safe for concurrent use.
func (lf *logFields) AddField(f zap.Field) {
	lf.mu.Lock()
	defer lf.mu.Unlock()
	lf.fields = append(lf.fields, f)
}

// Fields returns a copy of all accumulated fields.
func (lf *logFields) Fields() []zap.Field {
	lf.mu.Lock()
	defer lf.mu.Unlock()
	out := make([]zap.Field, len(lf.fields))
	copy(out, lf.fields)
	return out
}

// withLogFields stores a logFields holder in the context.
func withLogFields(ctx context.Context, lf *logFields) context.Context {
	return context.WithValue(ctx, logFieldsKey{}, lf)
}

// logFieldsFromContext retrieves the shared logFields holder, or nil if absent.
func logFieldsFromContext(ctx context.Context) *logFields {
	if lf, ok := ctx.Value(logFieldsKey{}).(*logFields); ok {
		return lf
	}
	return nil
}

// UnaryLogging returns a unary server interceptor that injects a
// request-scoped Zap logger into the context and logs request summaries.
// The logger carries request_id, rpc_method, and (if authenticated) user_id.
//
// A shared logFields holder is placed in the context so that downstream
// interceptors (e.g. auth) can add fields (like user_id) that appear in
// the completion log.
func UnaryLogging(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		lf := &logFields{}
		ctx = withLogFields(ctx, lf)
		ctx = enrichContext(ctx, logger, info.FullMethod)
		start := time.Now()

		resp, err := handler(ctx, req)

		logRequestSummary(ctx, lf, start, err)
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
		lf := &logFields{}
		ctx := withLogFields(ss.Context(), lf)
		ctx = enrichContext(ctx, logger, info.FullMethod)
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}
		start := time.Now()

		err := handler(srv, wrapped)

		logRequestSummary(wrapped.Context(), lf, start, err)
		return err
	}
}

// enrichContext adds request-scoped fields to the logger and injects it into context.
// Note: user_id is not available here because the auth interceptor runs after logging.
// The auth interceptor adds user_id to the shared logFields holder after authentication.
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
// It reads the context logger (which has rpc_method and request_id) and appends
// any fields added by downstream interceptors via the shared logFields holder
// (e.g. user_id from the auth interceptor).
func logRequestSummary(ctx context.Context, lf *logFields, start time.Time, err error) {
	logger := logging.FromContext(ctx)
	duration := time.Since(start)

	fields := []zap.Field{
		zap.Duration("duration", duration),
	}

	// Append fields from downstream interceptors (e.g. user_id from auth).
	if lf != nil {
		fields = append(fields, lf.Fields()...)
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
