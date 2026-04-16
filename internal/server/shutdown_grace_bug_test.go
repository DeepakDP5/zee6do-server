package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestShutdownManager_grace_period_expired_components_still_closed verifies that
// registered components are ALWAYS closed during Shutdown(), even when the grace
// period expires before the drain interval completes.
//
// Previously, an early `return` in the select statement meant components were
// silently skipped when gracePeriod < drainInterval, leaking DB connections.
func TestShutdownManager_grace_period_expired_components_still_closed(t *testing.T) {
	health := NewHealthChecker()
	gracePeriod := 30 * time.Millisecond   // shorter than drain
	drainInterval := 200 * time.Millisecond // longer than grace

	sm := NewShutdownManager(zap.NewNop(), health, drainInterval, gracePeriod)

	comp := &mockComponent{}
	sm.Register("mongodb", comp)

	sm.Shutdown()

	assert.True(t, comp.closed,
		"component MUST be closed even when grace period expires during drain wait")
	assert.False(t, health.IsHealthy(), "health must be marked unhealthy")
}
