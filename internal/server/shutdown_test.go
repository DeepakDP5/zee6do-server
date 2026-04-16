package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockComponent struct {
	closed    bool
	closeErr  error
	closeFunc func(ctx context.Context) error
}

func (m *mockComponent) Close(ctx context.Context) error {
	m.closed = true
	if m.closeFunc != nil {
		return m.closeFunc(ctx)
	}
	return m.closeErr
}

func TestShutdownManager_marks_health_unhealthy(t *testing.T) {
	health := NewHealthChecker()
	assert.True(t, health.IsHealthy())

	sm := NewShutdownManager(zap.NewNop(), health, 0, 5*time.Second)
	sm.Shutdown()

	assert.False(t, health.IsHealthy())
}

func TestShutdownManager_closes_components_in_order(t *testing.T) {
	health := NewHealthChecker()
	sm := NewShutdownManager(zap.NewNop(), health, 0, 5*time.Second)

	var order []string
	c1 := &mockComponent{closeFunc: func(_ context.Context) error {
		order = append(order, "first")
		return nil
	}}
	c2 := &mockComponent{closeFunc: func(_ context.Context) error {
		order = append(order, "second")
		return nil
	}}

	sm.Register("first", c1)
	sm.Register("second", c2)
	sm.Shutdown()

	assert.Equal(t, []string{"first", "second"}, order)
	assert.True(t, c1.closed)
	assert.True(t, c2.closed)
}

func TestShutdownManager_continues_on_component_error(t *testing.T) {
	health := NewHealthChecker()
	sm := NewShutdownManager(zap.NewNop(), health, 0, 5*time.Second)

	failing := &mockComponent{closeErr: errors.New("close failed")}
	healthy := &mockComponent{}

	sm.Register("failing", failing)
	sm.Register("healthy", healthy)
	sm.Shutdown()

	assert.True(t, failing.closed, "failing component should still be called")
	assert.True(t, healthy.closed, "healthy component should still be called after failure")
}

func TestShutdownManager_respects_drain_interval(t *testing.T) {
	health := NewHealthChecker()
	drainInterval := 100 * time.Millisecond

	sm := NewShutdownManager(zap.NewNop(), health, drainInterval, 5*time.Second)
	comp := &mockComponent{}
	sm.Register("comp", comp)

	start := time.Now()
	sm.Shutdown()
	elapsed := time.Since(start)

	assert.True(t, elapsed >= drainInterval,
		"shutdown should wait at least drain interval, elapsed: %v", elapsed)
	assert.True(t, comp.closed)
}

func TestShutdownManager_with_no_components(t *testing.T) {
	health := NewHealthChecker()
	sm := NewShutdownManager(zap.NewNop(), health, 0, 5*time.Second)

	// Should not panic
	sm.Shutdown()
	assert.False(t, health.IsHealthy())
}
