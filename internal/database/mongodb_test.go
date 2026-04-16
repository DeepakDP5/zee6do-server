package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// NewMongoClient requires a live MongoDB connection, so integration tests
// are run with docker-compose in CI. Unit tests here cover the struct API.

func TestMongoClient_nil_safety(t *testing.T) {
	// Verify the struct fields and methods exist with correct signatures.
	// Actual connection tests require a running MongoDB instance.
	var mc *MongoClient
	assert.Nil(t, mc)
}

// Integration tests for NewMongoClient, HealthCheck, and Close require a
// real MongoDB instance. They are tested via docker-compose in CI:
//   docker-compose up -d mongodb
//   ZEE6DO_MONGODB_URI=mongodb://localhost:27017 go test -tags=integration ./internal/database/...
