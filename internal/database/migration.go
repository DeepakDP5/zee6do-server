package database

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

const migrationsCollection = "_migrations"

// MigrationFunc is a function that performs a migration.
// It receives the database to operate on and must be idempotent.
type MigrationFunc func(ctx context.Context, db *mongo.Database) error

// Migration represents a single database migration.
type Migration struct {
	// ID is a unique identifier for this migration (e.g., "001_create_users_indexes").
	ID string
	// Description is a human-readable summary of what this migration does.
	Description string
	// Up performs the migration.
	Up MigrationFunc
}

// migrationRecord is stored in the _migrations collection to track applied migrations.
type migrationRecord struct {
	ID          string    `bson:"_id"`
	Description string    `bson:"description"`
	AppliedAt   time.Time `bson:"applied_at"`
}

// MigrationRunner manages and executes database migrations.
type MigrationRunner struct {
	db         *mongo.Database
	logger     *zap.Logger
	migrations []Migration
}

// NewMigrationRunner creates a migration runner for the given database.
func NewMigrationRunner(db *mongo.Database, logger *zap.Logger) *MigrationRunner {
	return &MigrationRunner{
		db:     db,
		logger: logger,
	}
}

// Register adds a migration to the runner. Migrations are executed in the
// order they are registered.
func (r *MigrationRunner) Register(m Migration) {
	r.migrations = append(r.migrations, m)
}

// Run executes all pending migrations in order. Each migration is checked
// against the _migrations collection -- if already applied, it is skipped.
// This makes migrations idempotent at the runner level.
func (r *MigrationRunner) Run(ctx context.Context) error {
	applied, err := r.getApplied(ctx)
	if err != nil {
		return fmt.Errorf("migrationRunner.Run: get applied: %w", err)
	}

	// Sort by ID for deterministic order
	sorted := make([]Migration, len(r.migrations))
	copy(sorted, r.migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	for _, m := range sorted {
		if applied[m.ID] {
			r.logger.Debug("migration already applied, skipping",
				zap.String("migration_id", m.ID),
			)
			continue
		}

		r.logger.Info("applying migration",
			zap.String("migration_id", m.ID),
			zap.String("description", m.Description),
		)

		if err := m.Up(ctx, r.db); err != nil {
			return fmt.Errorf("migrationRunner.Run: migration %q: %w", m.ID, err)
		}

		if err := r.markApplied(ctx, m); err != nil {
			return fmt.Errorf("migrationRunner.Run: mark applied %q: %w", m.ID, err)
		}

		r.logger.Info("migration applied successfully",
			zap.String("migration_id", m.ID),
		)
	}

	return nil
}

func (r *MigrationRunner) getApplied(ctx context.Context) (map[string]bool, error) {
	coll := r.db.Collection(migrationsCollection)

	cursor, err := coll.Find(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("find migrations: %w", err)
	}
	defer cursor.Close(ctx)

	applied := make(map[string]bool)
	for cursor.Next(ctx) {
		var record migrationRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, fmt.Errorf("decode migration record: %w", err)
		}
		applied[record.ID] = true
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return applied, nil
}

func (r *MigrationRunner) markApplied(ctx context.Context, m Migration) error {
	coll := r.db.Collection(migrationsCollection)

	record := migrationRecord{
		ID:          m.ID,
		Description: m.Description,
		AppliedAt:   time.Now().UTC(),
	}

	_, err := coll.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("insert migration record: %w", err)
	}

	return nil
}
