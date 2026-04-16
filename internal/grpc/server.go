// Package grpc provides the gRPC server setup and lifecycle management
// for the zee6do server. It wires the interceptor stack and implements
// the server.Shutdownable interface for graceful shutdown.
package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Compile-time check that Server implements server.Shutdownable.
// We use a Close(context.Context) error interface check to avoid
// importing internal/server (which would create a circular dependency).
var _ interface {
	Close(ctx context.Context) error
} = (*Server)(nil)

// Server wraps a gRPC server with lifecycle management.
type Server struct {
	grpcServer *grpc.Server
	logger     *zap.Logger
	port       int
}

// NewServer creates a new gRPC server with the interceptor stack wired in order:
// request_id → logging → recovery → auth → validation.
func NewServer(cfg *config.Config, logger *zap.Logger, authCfg middleware.AuthConfig) *Server {
	// Interceptor stack order matters: outermost (request_id) runs first.
	unaryInterceptors := grpc.ChainUnaryInterceptor(
		middleware.UnaryRequestID(),
		middleware.UnaryLogging(logger),
		middleware.UnaryRecovery(),
		middleware.UnaryAuth(authCfg),
		middleware.UnaryValidation(),
	)

	streamInterceptors := grpc.ChainStreamInterceptor(
		middleware.StreamRequestID(),
		middleware.StreamLogging(logger),
		middleware.StreamRecovery(),
		middleware.StreamAuth(authCfg),
		middleware.StreamValidation(),
	)

	grpcServer := grpc.NewServer(unaryInterceptors, streamInterceptors)

	return &Server{
		grpcServer: grpcServer,
		logger:     logger,
		port:       cfg.Server.GRPCPort,
	}
}

// GRPCServer returns the underlying *grpc.Server so that service
// implementations can be registered on it.
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Start begins listening on the configured port. This method blocks until
// the server is stopped. It should be called in a goroutine from main.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpc server: failed to listen on %s: %w", addr, err)
	}

	s.logger.Info("gRPC server listening", zap.String("addr", addr))
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("grpc server: serve failed: %w", err)
	}
	return nil
}

// Close performs a graceful stop of the gRPC server. It stops accepting new
// connections and waits for in-flight RPCs to complete. If the context
// deadline is exceeded, it forces an immediate stop.
//
// Note: GracefulStop() does not accept a context and blocks indefinitely,
// so a goroutine with bounded lifetime (capped by ctx cancellation) is used
// to enable timeout-based forced shutdown. This is the standard gRPC-Go pattern.
func (s *Server) Close(ctx context.Context) error {
	s.logger.Info("gRPC server: initiating graceful stop")

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		s.logger.Info("gRPC server: graceful stop complete")
	case <-ctx.Done():
		s.logger.Warn("gRPC server: graceful stop timed out, forcing stop")
		s.grpcServer.Stop()
	}

	return nil
}
