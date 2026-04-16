package server

import (
	"sync/atomic"
)

// HealthChecker tracks the server's health status. During graceful shutdown,
// the health check is marked unhealthy so the load balancer stops routing
// new requests before the server stops.
type HealthChecker struct {
	healthy atomic.Bool
}

// NewHealthChecker creates a new health checker in healthy state.
func NewHealthChecker() *HealthChecker {
	h := &HealthChecker{}
	h.healthy.Store(true)
	return h
}

// IsHealthy reports whether the server is healthy and accepting requests.
func (h *HealthChecker) IsHealthy() bool {
	return h.healthy.Load()
}

// SetUnhealthy marks the server as unhealthy. Called as the first step
// of graceful shutdown.
func (h *HealthChecker) SetUnhealthy() {
	h.healthy.Store(false)
}
