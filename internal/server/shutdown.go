// Package server provides the application server lifecycle management
// including health checking and graceful shutdown.
package server

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Shutdownable represents a component that can be shut down gracefully.
type Shutdownable interface {
	Close(ctx context.Context) error
}

// ShutdownManager orchestrates the graceful shutdown sequence.
// The sequence follows the exact order specified in the architecture docs:
//  1. Mark health check as unhealthy
//  2. Wait for load balancer drain interval
//  3. (Future: drain WebSocket connections)
//  4. (Future: stop worker pools)
//  5. (Future: stop gRPC server)
//  6. Flush logs
//  7. Close database connections
//  8. Exit
type ShutdownManager struct {
	logger        *zap.Logger
	health        *HealthChecker
	drainInterval time.Duration
	gracePeriod   time.Duration
	components    []namedComponent
}

type namedComponent struct {
	name      string
	component Shutdownable
}

// NewShutdownManager creates a shutdown manager with the configured timings.
func NewShutdownManager(
	logger *zap.Logger,
	health *HealthChecker,
	drainInterval time.Duration,
	gracePeriod time.Duration,
) *ShutdownManager {
	return &ShutdownManager{
		logger:        logger,
		health:        health,
		drainInterval: drainInterval,
		gracePeriod:   gracePeriod,
	}
}

// Register adds a component to be shut down. Components are closed in the
// order they are registered.
func (s *ShutdownManager) Register(name string, component Shutdownable) {
	s.components = append(s.components, namedComponent{
		name:      name,
		component: component,
	})
}

// Shutdown executes the graceful shutdown sequence. It returns after all
// components are closed or the grace period expires, whichever comes first.
// Components are always closed, even if the drain wait is cut short by the
// grace period expiring.
func (s *ShutdownManager) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), s.gracePeriod)
	defer cancel()

	// Step 1: Mark unhealthy
	s.logger.Info("shutdown: marking health check as unhealthy")
	s.health.SetUnhealthy()

	// Step 2: Wait for load balancer drain (bounded by grace period).
	// If the grace period expires during drain, we log a warning and proceed
	// immediately to closing components rather than abandoning them.
	s.logger.Info("shutdown: waiting for load balancer drain",
		zap.Duration("drain_interval", s.drainInterval),
	)
	select {
	case <-time.After(s.drainInterval):
	case <-ctx.Done():
		s.logger.Warn("shutdown: grace period expired during drain wait, proceeding to close components")
	}

	// Steps 3-5 are placeholders for future components:
	// - WebSocket drain (will be registered as a Shutdownable)
	// - Worker pool stop (will be registered as a Shutdownable)
	// - gRPC server stop (will be registered as a Shutdownable)

	// Steps 6-7: Close registered components (logger flush, DB, etc.).
	// Components always run, even if context is already expired, so that
	// resources (DB connections, file handles) are always released on shutdown.
	for _, nc := range s.components {
		s.logger.Info("shutdown: closing component", zap.String("component", nc.name))
		if err := nc.component.Close(ctx); err != nil {
			s.logger.Error("shutdown: failed to close component",
				zap.String("component", nc.name),
				zap.Error(err),
			)
		} else {
			s.logger.Info("shutdown: component closed", zap.String("component", nc.name))
		}
	}

	s.logger.Info("shutdown: complete")
}
