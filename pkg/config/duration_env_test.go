package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestLoad_duration_from_env_var verifies that Viper correctly parses Go duration
// strings (e.g. "45s", "60s") supplied via environment variables — a non-obvious
// behaviour that was confirmed during testing and is worth protecting.
func TestLoad_duration_from_env_var(t *testing.T) {
	t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("ZEE6DO_MONGODB_TIMEOUT", "45s")
	t.Setenv("ZEE6DO_SHUTDOWN_GRACE_PERIOD", "60s")

	cfg := LoadFromEnv()

	assert.Equal(t, 45*time.Second, cfg.MongoDB.Timeout,
		"duration string env var ZEE6DO_MONGODB_TIMEOUT should parse to 45s")
	assert.Equal(t, 60*time.Second, cfg.Shutdown.GracePeriod,
		"duration string env var ZEE6DO_SHUTDOWN_GRACE_PERIOD should parse to 60s")
}
