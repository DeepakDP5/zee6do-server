package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthChecker_starts_healthy(t *testing.T) {
	h := NewHealthChecker()
	assert.True(t, h.IsHealthy())
}

func TestHealthChecker_can_be_marked_unhealthy(t *testing.T) {
	h := NewHealthChecker()
	h.SetUnhealthy()
	assert.False(t, h.IsHealthy())
}

func TestHealthChecker_set_unhealthy_is_idempotent(t *testing.T) {
	h := NewHealthChecker()
	h.SetUnhealthy()
	h.SetUnhealthy()
	assert.False(t, h.IsHealthy())
}
