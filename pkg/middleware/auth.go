package middleware

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type userIDKey struct{}

// JWTValidator is the interface for JWT token validation. The auth module
// provides the real implementation; during bootstrap a placeholder that
// accepts any well-formed token can be used.
type JWTValidator interface {
	// ValidateToken validates the given JWT token string and returns the user ID
	// embedded in its claims. Returns an error if the token is invalid or expired.
	ValidateToken(ctx context.Context, token string) (userID string, err error)
}

// UserIDFromContext extracts the authenticated user ID from the context.
// Returns an empty string if no user ID is set (e.g., unauthenticated RPC).
func UserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(userIDKey{}).(string); ok {
		return id
	}
	return ""
}

// withUserID returns a context with the user ID set.
func withUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// AuthConfig configures the auth interceptor.
type AuthConfig struct {
	// Validator performs JWT token validation.
	Validator JWTValidator
	// SkipMethods is the set of full gRPC method names that bypass authentication.
	// Keys are full method names like "/zee6do.v1.AuthService/SendOTP".
	SkipMethods map[string]bool
}

// UnaryAuth returns a unary server interceptor that extracts and validates
// a JWT from the "authorization" metadata header. On success the user_id
// from the token claims is injected into the context.
//
// Methods listed in SkipMethods bypass authentication entirely.
func UnaryAuth(cfg AuthConfig) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if cfg.SkipMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		ctx, err := authenticate(ctx, cfg.Validator)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamAuth returns a stream server interceptor that extracts and validates
// a JWT from the "authorization" metadata header.
func StreamAuth(cfg AuthConfig) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if cfg.SkipMethods[info.FullMethod] {
			return handler(srv, ss)
		}

		ctx, err := authenticate(ss.Context(), cfg.Validator)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
	}
}

// authenticate extracts the bearer token from metadata, validates it via
// the JWTValidator, and returns a context with the user_id set.
func authenticate(ctx context.Context, validator JWTValidator) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, status.Error(codes.Unauthenticated, "missing metadata")
	}

	authValues := md.Get("authorization")
	if len(authValues) == 0 || authValues[0] == "" {
		return ctx, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	token := authValues[0]
	// Strip "Bearer " prefix if present.
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = token[7:]
	}

	if token == "" {
		return ctx, status.Error(codes.Unauthenticated, "empty token")
	}

	userID, err := validator.ValidateToken(ctx, token)
	if err != nil {
		return ctx, status.Error(codes.Unauthenticated, "invalid token")
	}

	ctx = withUserID(ctx, userID)

	// Add user_id to the shared log-fields holder so the logging interceptor's
	// completion log includes the authenticated user, even though it holds
	// the parent context (not auth's enriched child context).
	if lf := logFieldsFromContext(ctx); lf != nil {
		lf.AddField(zap.String("user_id", userID))
	}

	return ctx, nil
}
