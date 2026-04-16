//go:build integration

package migrations_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DeepakDP5/zee6do-server/internal/database"
	"github.com/DeepakDP5/zee6do-server/migrations"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.uber.org/zap"
)

func testMigrationsConfig(t *testing.T) *config.Config {
	t.Helper()
	uri := os.Getenv("ZEE6DO_MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	return &config.Config{
		Server:  config.ServerConfig{GRPCPort: 50051, Environment: "development"},
		MongoDB: config.MongoDBConfig{
			URI:                    uri,
			Database:               "zee6do_migrations_integration_test",
			MaxPoolSize:            10,
			ConnectTimeout:         5 * time.Second,
			Timeout:                10 * time.Second,
			ServerSelectionTimeout: 3 * time.Second,
		},
		Logging:  config.LoggingConfig{Level: "info"},
		Shutdown: config.ShutdownConfig{GracePeriod: 30 * time.Second, DrainInterval: 5 * time.Second},
	}
}

// indexHasField returns true if bson.D (as returned by the MongoDB driver's Indexes().List())
// contains the given field key. The index "key" document is bson.D, not bson.M.
func indexHasField(idx bson.M, field string) bool {
	keys, ok := idx["key"].(bson.D)
	if !ok {
		return false
	}
	for _, elem := range keys {
		if elem.Key == field {
			return true
		}
	}
	return false
}

// T12: Verify 001_create_initial_indexes runs successfully and creates expected indexes.
func TestRegister_and_Run_initial_indexes(t *testing.T) {
	cfg := testMigrationsConfig(t)
	ctx := context.Background()
	logger := zap.NewNop()
	mc, err := database.NewMongoClient(ctx, cfg, logger)
	require.NoError(t, err)
	defer func() {
		_ = mc.Client().Database(cfg.MongoDB.Database).Drop(ctx)
		_ = mc.Close(ctx)
	}()

	runner := database.NewMigrationRunner(mc.Database(), logger)
	migrations.Register(runner)
	err = runner.Run(ctx)
	require.NoError(t, err, "initial migrations should run without error")

	db := mc.Database()

	// Verify the migration record was written to _migrations
	coll := db.Collection("_migrations")
	var record bson.M
	err = coll.FindOne(ctx, bson.D{{Key: "_id", Value: "001_create_initial_indexes"}}).Decode(&record)
	require.NoError(t, err, "migration record 001_create_initial_indexes should exist in _migrations")
	assert.Equal(t, "Create initial indexes for core collections", record["description"])

	// Verify unique index on users.user_id (bson.D key documents, not bson.M)
	userIndexes, err := db.Collection("users").Indexes().List(ctx)
	require.NoError(t, err)
	var userIdxDocs []bson.M
	require.NoError(t, userIndexes.All(ctx, &userIdxDocs))
	var foundUserUniqueIdx bool
	for _, idx := range userIdxDocs {
		if indexHasField(idx, "user_id") {
			if unique, ok := idx["unique"].(bool); ok && unique {
				foundUserUniqueIdx = true
			}
		}
	}
	assert.True(t, foundUserUniqueIdx, "users collection should have a unique index on user_id")

	// Verify compound index on tasks (user_id, status, priority)
	taskIndexes, err := db.Collection("tasks").Indexes().List(ctx)
	require.NoError(t, err)
	var taskIdxDocs []bson.M
	require.NoError(t, taskIndexes.All(ctx, &taskIdxDocs))
	var foundTaskCompoundIdx bool
	for _, idx := range taskIdxDocs {
		if indexHasField(idx, "user_id") && indexHasField(idx, "status") && indexHasField(idx, "priority") {
			foundTaskCompoundIdx = true
		}
	}
	assert.True(t, foundTaskCompoundIdx, "tasks collection should have a compound index on (user_id, status, priority)")

	// T10 (migrations idempotency): run again — no error, no double-apply
	err = runner.Run(ctx)
	assert.NoError(t, err, "running migrations a second time must be idempotent")
	count, err := coll.CountDocuments(ctx, bson.D{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "_migrations should still have exactly one record after second run")
}
