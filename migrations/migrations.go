// Package migrations registers all database migrations for the zee6do server.
// Migrations are auto-applied on startup via the MigrationRunner.
//
// To add a new migration:
//  1. Create a new function following the naming pattern: migrate_NNN_description
//  2. Register it in the Register function with a unique sequential ID
//  3. The migration function must be idempotent
package migrations

import (
	"context"
	"fmt"

	"github.com/DeepakDP5/zee6do-server/internal/database"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Register adds all migrations to the runner.
func Register(runner *database.MigrationRunner) {
	runner.Register(database.Migration{
		ID:          "001_create_initial_indexes",
		Description: "Create initial indexes for core collections",
		Up:          migrateCreateInitialIndexes,
	})
	runner.Register(database.Migration{
		ID:          "002_auth_indexes",
		Description: "Create indexes for auth module (sessions, otp_records, users)",
		Up:          migration002AuthIndexes,
	})
}

func migrateCreateInitialIndexes(ctx context.Context, db *mongo.Database) error {
	// Users collection: unique index on user_id
	_, err := db.Collection("users").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "user_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("create users.user_id unique index: %w", err)
	}

	// Tasks collection: compound index for user task queries
	_, err = db.Collection("tasks").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "status", Value: 1},
			{Key: "priority", Value: 1},
		},
	})
	if err != nil {
		return fmt.Errorf("create tasks.(user_id,status,priority) compound index: %w", err)
	}

	return nil
}
