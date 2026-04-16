// Package logging provides structured JSON logging via Uber Zap.
// It supports request-scoped loggers injected into context, ensuring
// every log entry carries request_id, user_id, and rpc_method when available.
//
// Usage:
//
//	logger := logging.FromContext(ctx)
//	logger.Info("task created", zap.String("task_id", id))
package logging

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey struct{}

// NewLogger creates a new production Zap logger with structured JSON output.
// It panics if the logger cannot be built (broken config).
func NewLogger(level string) *zap.Logger {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}

	cfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(lvl),
		Development: false,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.MillisDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := cfg.Build()
	if err != nil {
		panic("logging: failed to build logger: " + err.Error())
	}

	return logger
}

// NewDevelopmentLogger creates a logger suitable for local development
// with human-readable console output.
func NewDevelopmentLogger() *zap.Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic("logging: failed to build development logger: " + err.Error())
	}
	return logger
}

// WithContext returns a new context with the given logger attached.
func WithContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext extracts the logger from the context.
// Returns a no-op logger if none is found (never nil).
func FromContext(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*zap.Logger); ok {
		return logger
	}
	return zap.NewNop()
}

// WithFields returns a new context whose logger has the given fields attached.
// Useful for adding request-scoped metadata (request_id, user_id, rpc_method).
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	logger := FromContext(ctx)
	return WithContext(ctx, logger.With(fields...))
}
