package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_from_yaml(t *testing.T) {
	yamlContent := `
server:
  grpc_port: 9090
  environment: staging
mongodb:
  uri: "mongodb://localhost:27017"
  database: "testdb"
  max_pool_size: 50
logging:
  level: debug
shutdown:
  grace_period: 15s
  drain_interval: 5s
jwt:
  secret: "test-secret-staging"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0o600))

	cfg := Load(cfgPath)

	assert.Equal(t, 9090, cfg.Server.GRPCPort)
	assert.Equal(t, "staging", cfg.Server.Environment)
	assert.Equal(t, "mongodb://localhost:27017", cfg.MongoDB.URI)
	assert.Equal(t, "testdb", cfg.MongoDB.Database)
	assert.Equal(t, uint64(50), cfg.MongoDB.MaxPoolSize)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, 15*time.Second, cfg.Shutdown.GracePeriod)
	assert.Equal(t, 5*time.Second, cfg.Shutdown.DrainInterval)
	assert.Equal(t, "test-secret-staging", cfg.JWT.Secret)
}

func TestLoad_env_vars_override_yaml(t *testing.T) {
	yamlContent := `
server:
  grpc_port: 9090
  environment: staging
mongodb:
  uri: "mongodb://yaml-host:27017"
  database: "yamldb"
logging:
  level: info
jwt:
  secret: "test-secret"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0o600))

	t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://env-host:27017")
	t.Setenv("ZEE6DO_LOGGING_LEVEL", "error")

	cfg := Load(cfgPath)

	assert.Equal(t, "mongodb://env-host:27017", cfg.MongoDB.URI)
	assert.Equal(t, "error", cfg.Logging.Level)
	// Non-overridden values stay from YAML
	assert.Equal(t, 9090, cfg.Server.GRPCPort)
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("ZEE6DO_SERVER_GRPC_PORT", "8080")
	t.Setenv("ZEE6DO_SERVER_ENVIRONMENT", "production")
	t.Setenv("ZEE6DO_JWT_SECRET", "test-jwt-secret")

	cfg := LoadFromEnv()

	assert.Equal(t, "mongodb://localhost:27017", cfg.MongoDB.URI)
	assert.Equal(t, 8080, cfg.Server.GRPCPort)
	assert.Equal(t, "production", cfg.Server.Environment)
	assert.Equal(t, "test-jwt-secret", cfg.JWT.Secret)
}

func TestLoad_defaults(t *testing.T) {
	t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://localhost:27017")

	cfg := LoadFromEnv()

	assert.Equal(t, 50051, cfg.Server.GRPCPort)
	assert.Equal(t, "development", cfg.Server.Environment)
	assert.Equal(t, "zee6do", cfg.MongoDB.Database)
	assert.Equal(t, uint64(100), cfg.MongoDB.MaxPoolSize)
	assert.Equal(t, 10*time.Second, cfg.MongoDB.ConnectTimeout)
	assert.Equal(t, 30*time.Second, cfg.MongoDB.Timeout)
	assert.Equal(t, 5*time.Second, cfg.MongoDB.ServerSelectionTimeout)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, 30*time.Second, cfg.Shutdown.GracePeriod)
	assert.Equal(t, 10*time.Second, cfg.Shutdown.DrainInterval)
	// JWT defaults
	assert.Equal(t, 15*time.Minute, cfg.JWT.AccessTTL)
	assert.Equal(t, 30*24*time.Hour, cfg.JWT.RefreshTTL) // 30 days
	// Rate limiting defaults
	assert.False(t, cfg.RateLimiting.Enabled)
	assert.Equal(t, 100, cfg.RateLimiting.DefaultRPS)
}

func TestLoad_panics_on_missing_mongodb_uri(t *testing.T) {
	// Clear any env var that might provide the URI
	t.Setenv("ZEE6DO_MONGODB_URI", "")

	assert.Panics(t, func() {
		LoadFromEnv()
	}, "should panic when mongodb.uri is missing")
}

func TestLoad_panics_on_invalid_port(t *testing.T) {
	for _, tc := range []struct{ name, port, reason string }{
		{"zero", "0", "port is 0"},
		{"negative", "-1", "port is negative"},
		{"above_max", "70000", "port exceeds 65535"},
		{"boundary", "65536", "port is exactly above max"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://localhost:27017")
			t.Setenv("ZEE6DO_SERVER_GRPC_PORT", tc.port)
			assert.Panics(t, func() { LoadFromEnv() }, "should panic when "+tc.reason)
		})
	}
}

func TestLoad_panics_on_invalid_environment(t *testing.T) {
	t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("ZEE6DO_SERVER_ENVIRONMENT", "invalid_env")

	assert.Panics(t, func() {
		LoadFromEnv()
	}, "should panic on invalid environment")
}

func TestLoad_panics_on_missing_jwt_secret_in_staging(t *testing.T) {
	t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("ZEE6DO_SERVER_ENVIRONMENT", "staging")
	// Deliberately not setting ZEE6DO_JWT_SECRET

	assert.PanicsWithValue(t,
		"config: missing required configuration: jwt.secret (env: ZEE6DO_JWT_SECRET) — required in staging/production",
		func() { LoadFromEnv() },
		"should panic when jwt.secret is missing in staging",
	)
}

func TestLoad_panics_on_missing_jwt_secret_in_production(t *testing.T) {
	t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("ZEE6DO_SERVER_ENVIRONMENT", "production")

	assert.PanicsWithValue(t,
		"config: missing required configuration: jwt.secret (env: ZEE6DO_JWT_SECRET) — required in staging/production",
		func() { LoadFromEnv() },
		"should panic when jwt.secret is missing in production",
	)
}

func TestLoad_no_panic_without_jwt_secret_in_development(t *testing.T) {
	t.Setenv("ZEE6DO_MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("ZEE6DO_SERVER_ENVIRONMENT", "development")

	assert.NotPanics(t, func() {
		LoadFromEnv()
	}, "development environment should not require jwt.secret")
}

func TestLoad_panics_on_missing_config_file(t *testing.T) {
	assert.Panics(t, func() {
		Load("/nonexistent/config.yaml")
	}, "should panic when config file path is given but file doesn't exist")
}
