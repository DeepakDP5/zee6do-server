package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewMigrationRunner(t *testing.T) {
	runner := NewMigrationRunner(nil, zap.NewNop())
	require.NotNil(t, runner)
	assert.Empty(t, runner.migrations)
}

func TestMigrationRunner_Register(t *testing.T) {
	runner := NewMigrationRunner(nil, zap.NewNop())

	runner.Register(Migration{
		ID:          "001_create_users",
		Description: "Create users collection indexes",
		Up:          nil,
	})
	runner.Register(Migration{
		ID:          "002_create_tasks",
		Description: "Create tasks collection indexes",
		Up:          nil,
	})

	assert.Len(t, runner.migrations, 2)
	assert.Equal(t, "001_create_users", runner.migrations[0].ID)
	assert.Equal(t, "002_create_tasks", runner.migrations[1].ID)
}

func TestMigrationRunner_Register_preserves_order(t *testing.T) {
	runner := NewMigrationRunner(nil, zap.NewNop())

	// Register out of ID order
	runner.Register(Migration{ID: "003_third"})
	runner.Register(Migration{ID: "001_first"})
	runner.Register(Migration{ID: "002_second"})

	// Registration order is preserved (execution order is sorted by ID in Run)
	assert.Equal(t, "003_third", runner.migrations[0].ID)
	assert.Equal(t, "001_first", runner.migrations[1].ID)
	assert.Equal(t, "002_second", runner.migrations[2].ID)
}

func TestMigration_struct_fields(t *testing.T) {
	m := Migration{
		ID:          "001_test",
		Description: "Test migration",
		Up:          nil,
	}

	assert.Equal(t, "001_test", m.ID)
	assert.Equal(t, "Test migration", m.Description)
	assert.Nil(t, m.Up)
}

// Integration tests for Run() require a real MongoDB instance.
// They are tested via docker-compose in CI:
//   docker-compose up -d mongodb
//   go test -tags=integration ./internal/database/...
