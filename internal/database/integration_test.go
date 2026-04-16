//go:build integration

package database_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DeepakDP5/zee6do-server/internal/database"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	uri := os.Getenv("ZEE6DO_MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	return &config.Config{
		Server: config.ServerConfig{
			GRPCPort:    50051,
			Environment: "development",
		},
		MongoDB: config.MongoDBConfig{
			URI:                    uri,
			Database:               "zee6do_integration_test",
			MaxPoolSize:            10,
			ConnectTimeout:         5 * time.Second,
			Timeout:                10 * time.Second,
			ServerSelectionTimeout: 3 * time.Second,
		},
		Logging:  config.LoggingConfig{Level: "info"},
		Shutdown: config.ShutdownConfig{GracePeriod: 30 * time.Second, DrainInterval: 5 * time.Second},
	}
}

// T9: Real MongoDB connection — connect, ping, disconnect
func TestNewMongoClient_connect_healthcheck_close(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	logger := zap.NewNop()

	mc, err := database.NewMongoClient(ctx, cfg, logger)
	require.NoError(t, err, "NewMongoClient should connect successfully")
	require.NotNil(t, mc)
	require.NotNil(t, mc.Database())
	require.NotNil(t, mc.Client())

	// T9: HealthCheck
	err = mc.HealthCheck(ctx)
	assert.NoError(t, err, "HealthCheck should succeed on live MongoDB")

	// T9: Close
	closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = mc.Close(closeCtx)
	assert.NoError(t, err, "Close should disconnect cleanly")
}

// T10: Migration idempotency — run twice, migration applied only once
func TestMigrationRunner_Run_idempotent(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	// Use a fresh database name to isolate this test
	cfg.MongoDB.Database = "zee6do_migration_idempotency_test"
	logger := zap.NewNop()

	mc, err := database.NewMongoClient(ctx, cfg, logger)
	require.NoError(t, err)
	defer func() {
		// Drop the test database for cleanup
		_ = mc.Client().Database(cfg.MongoDB.Database).Drop(ctx)
		_ = mc.Close(ctx)
	}()

	db := mc.Database()

	var callCount int
	runner := database.NewMigrationRunner(db, logger)
	runner.Register(database.Migration{
		ID:          "001_test_idempotency",
		Description: "Test that this runs exactly once",
		Up: func(ctx context.Context, db *mongo.Database) error {
			callCount++
			return nil
		},
	})

	// First run: should apply the migration
	err = runner.Run(ctx)
	require.NoError(t, err, "first Run should succeed")
	assert.Equal(t, 1, callCount, "migration Up() should have been called once")

	// Second run: should skip (already applied)
	err = runner.Run(ctx)
	require.NoError(t, err, "second Run should succeed (idempotent)")
	assert.Equal(t, 1, callCount, "migration Up() should NOT be called again (idempotent)")

	// Verify _migrations collection has exactly one record
	coll := db.Collection("_migrations")
	count, err := coll.CountDocuments(ctx, bson.D{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "exactly one migration record in _migrations")
}

// T11: Multi-migration ordering — registered out of order, executed in ID sort order
func TestMigrationRunner_Run_sorts_by_id(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	cfg.MongoDB.Database = "zee6do_migration_order_test"
	logger := zap.NewNop()

	mc, err := database.NewMongoClient(ctx, cfg, logger)
	require.NoError(t, err)
	defer func() {
		_ = mc.Client().Database(cfg.MongoDB.Database).Drop(ctx)
		_ = mc.Close(ctx)
	}()

	db := mc.Database()

	var executionOrder []string
	runner := database.NewMigrationRunner(db, logger)

	// Register out of order
	runner.Register(database.Migration{
		ID: "003_third",
		Up: func(ctx context.Context, db *mongo.Database) error {
			executionOrder = append(executionOrder, "003")
			return nil
		},
	})
	runner.Register(database.Migration{
		ID: "001_first",
		Up: func(ctx context.Context, db *mongo.Database) error {
			executionOrder = append(executionOrder, "001")
			return nil
		},
	})
	runner.Register(database.Migration{
		ID: "002_second",
		Up: func(ctx context.Context, db *mongo.Database) error {
			executionOrder = append(executionOrder, "002")
			return nil
		},
	})

	err = runner.Run(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"001", "002", "003"}, executionOrder,
		"migrations must execute in lexicographic ID order regardless of registration order")
}

// T11 continued: Verify run fails gracefully when a migration Up() returns an error
func TestMigrationRunner_Run_stops_on_error(t *testing.T) {
	ctx := context.Background()
	cfg := testConfig(t)
	cfg.MongoDB.Database = "zee6do_migration_error_test"
	logger := zap.NewNop()

	mc, err := database.NewMongoClient(ctx, cfg, logger)
	require.NoError(t, err)
	defer func() {
		_ = mc.Client().Database(cfg.MongoDB.Database).Drop(ctx)
		_ = mc.Close(ctx)
	}()

	db := mc.Database()

	var secondCalled bool
	runner := database.NewMigrationRunner(db, logger)
	runner.Register(database.Migration{
		ID: "001_fails",
		Up: func(ctx context.Context, db *mongo.Database) error {
			return assert.AnError
		},
	})
	runner.Register(database.Migration{
		ID: "002_should_not_run",
		Up: func(ctx context.Context, db *mongo.Database) error {
			secondCalled = true
			return nil
		},
	})

	err = runner.Run(ctx)
	assert.Error(t, err, "Run should return the error from the failing migration")
	assert.Contains(t, err.Error(), "001_fails", "error should name the failing migration")
	assert.False(t, secondCalled, "migration after failure should not be executed")
}
