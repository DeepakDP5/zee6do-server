// Package config loads application configuration from environment variables
// and YAML files using Viper. All config is parsed into typed Go structs
// at startup and validated. Missing required values cause a startup panic
// with a clear error message.
//
// Config structs are injected via Wire, not stored as globals.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	MongoDB      MongoDBConfig      `mapstructure:"mongodb"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Shutdown     ShutdownConfig     `mapstructure:"shutdown"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	RateLimiting RateLimitingConfig `mapstructure:"rate_limiting"`
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	// GRPCPort is the port the gRPC server listens on.
	GRPCPort int `mapstructure:"grpc_port"`
	// Environment is the deployment environment (development, staging, production).
	Environment string `mapstructure:"environment"`
}

// MongoDBConfig holds MongoDB connection configuration.
type MongoDBConfig struct {
	// URI is the MongoDB connection string.
	URI string `mapstructure:"uri"`
	// Database is the database name.
	Database string `mapstructure:"database"`
	// MaxPoolSize is the maximum number of connections in the pool.
	MaxPoolSize uint64 `mapstructure:"max_pool_size"`
	// ConnectTimeout is the timeout for initial connection.
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
	// Timeout is the client-level operation timeout (maps to mongo-driver v2 SetTimeout).
	Timeout time.Duration `mapstructure:"timeout"`
	// ServerSelectionTimeout is the timeout for server selection.
	ServerSelectionTimeout time.Duration `mapstructure:"server_selection_timeout"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string `mapstructure:"level"`
}

// ShutdownConfig holds graceful shutdown configuration.
type ShutdownConfig struct {
	// GracePeriod is the total time allowed for graceful shutdown.
	GracePeriod time.Duration `mapstructure:"grace_period"`
	// DrainInterval is the time to wait for load balancer drain.
	DrainInterval time.Duration `mapstructure:"drain_interval"`
}

// JWTConfig holds JWT authentication configuration.
type JWTConfig struct {
	// Secret is the signing key for JWT tokens.
	Secret string `mapstructure:"secret"`
	// AccessTTL is the lifetime of access tokens.
	AccessTTL time.Duration `mapstructure:"access_ttl"`
	// RefreshTTL is the lifetime of refresh tokens.
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

// RateLimitingConfig holds rate limiting configuration (stub for future use).
type RateLimitingConfig struct {
	// Enabled controls whether rate limiting is active.
	Enabled bool `mapstructure:"enabled"`
	// DefaultRPS is the default requests-per-second limit per client.
	DefaultRPS int `mapstructure:"default_rps"`
}

// Load reads configuration from the given YAML file path and environment variables.
// Environment variables take precedence over file values.
// Panics if required configuration is missing or invalid.
func Load(configPath string) *Config {
	v := viper.New()

	setDefaults(v)
	bindEnvVars(v)

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			panic(fmt.Sprintf("config: failed to read config file %q: %v", configPath, err))
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		panic(fmt.Sprintf("config: failed to unmarshal configuration: %v", err))
	}

	validate(cfg)

	return cfg
}

// LoadFromEnv reads configuration from environment variables only (no file).
// Useful for containerized deployments where config comes from env vars.
func LoadFromEnv() *Config {
	return Load("")
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.grpc_port", 50051)
	v.SetDefault("server.environment", "development")

	v.SetDefault("mongodb.database", "zee6do")
	v.SetDefault("mongodb.max_pool_size", 100)
	v.SetDefault("mongodb.connect_timeout", 10*time.Second)
	v.SetDefault("mongodb.timeout", 30*time.Second)
	v.SetDefault("mongodb.server_selection_timeout", 5*time.Second)

	v.SetDefault("logging.level", "info")

	v.SetDefault("shutdown.grace_period", 30*time.Second)
	v.SetDefault("shutdown.drain_interval", 10*time.Second)

	v.SetDefault("jwt.access_ttl", 15*time.Minute)
	v.SetDefault("jwt.refresh_ttl", 30*24*time.Hour) // 30 days

	v.SetDefault("rate_limiting.enabled", false)
	v.SetDefault("rate_limiting.default_rps", 100)
}

// bindEnvVars explicitly binds each config key to its environment variable.
// This avoids issues with Viper's AutomaticEnv and nested key replacers.
// Format: ZEE6DO_<SECTION>_<KEY>
func bindEnvVars(v *viper.Viper) {
	bindings := map[string]string{ //nolint:gosec // G101 false positive: these are env var names, not credentials
		"server.grpc_port":                  "ZEE6DO_SERVER_GRPC_PORT",
		"server.environment":                "ZEE6DO_SERVER_ENVIRONMENT",
		"mongodb.uri":                       "ZEE6DO_MONGODB_URI",
		"mongodb.database":                  "ZEE6DO_MONGODB_DATABASE",
		"mongodb.max_pool_size":             "ZEE6DO_MONGODB_MAX_POOL_SIZE",
		"mongodb.connect_timeout":           "ZEE6DO_MONGODB_CONNECT_TIMEOUT",
		"mongodb.timeout":                   "ZEE6DO_MONGODB_TIMEOUT",
		"mongodb.server_selection_timeout":  "ZEE6DO_MONGODB_SERVER_SELECTION_TIMEOUT",
		"logging.level":                     "ZEE6DO_LOGGING_LEVEL",
		"shutdown.grace_period":             "ZEE6DO_SHUTDOWN_GRACE_PERIOD",
		"shutdown.drain_interval":           "ZEE6DO_SHUTDOWN_DRAIN_INTERVAL",
		"jwt.secret":                        "ZEE6DO_JWT_SECRET",
		"jwt.access_ttl":                    "ZEE6DO_JWT_ACCESS_TTL",
		"jwt.refresh_ttl":                   "ZEE6DO_JWT_REFRESH_TTL",
		"rate_limiting.enabled":             "ZEE6DO_RATE_LIMITING_ENABLED",
		"rate_limiting.default_rps":         "ZEE6DO_RATE_LIMITING_DEFAULT_RPS",
	}

	for key, env := range bindings {
		if err := v.BindEnv(key, env); err != nil {
			panic(fmt.Sprintf("config: failed to bind env var %s to %s: %v", env, key, err))
		}
	}
}

func validate(cfg *Config) {
	if cfg.MongoDB.URI == "" {
		panic("config: missing required configuration: mongodb.uri (env: ZEE6DO_MONGODB_URI)")
	}

	if cfg.Server.GRPCPort <= 0 || cfg.Server.GRPCPort > 65535 {
		panic(fmt.Sprintf("config: invalid server.grpc_port: %d (must be 1-65535)", cfg.Server.GRPCPort))
	}

	env := cfg.Server.Environment
	if env != "development" && env != "staging" && env != "production" {
		panic(fmt.Sprintf("config: invalid server.environment: %q (must be development, staging, or production)", env))
	}

	// JWT secret is required in non-development environments.
	if env != "development" && cfg.JWT.Secret == "" {
		panic("config: missing required configuration: jwt.secret (env: ZEE6DO_JWT_SECRET) — required in staging/production")
	}
}
