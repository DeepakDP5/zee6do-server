package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestShutdownManager_component_close_called_with_valid_ctx verifies that the
// context passed to component.Close() still has a valid (non-expired) deadline
// when gracePeriod is comfortably longer than drainInterval.
func TestShutdownManager_component_close_called_with_valid_ctx(t *testing.T) {
	health := NewHealthChecker()
	drainInterval := 10 * time.Millisecond
	gracePeriod := 5 * time.Second

	sm := NewShutdownManager(zap.NewNop(), health, drainInterval, gracePeriod)

	var ctxDeadlineOk bool
	comp := &mockComponent{
		closeFunc: func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			ctxDeadlineOk = ok && time.Until(deadline) > 0
			return nil
		},
	}
	sm.Register("mongodb", comp)
	sm.Shutdown()

	assert.True(t, comp.closed)
	assert.True(t, ctxDeadlineOk, "component Close() should receive a context with remaining deadline")
}
