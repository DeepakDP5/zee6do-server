package database

import (
	"github.com/DeepakDP5/zee6do-server/internal/server"
)

// Compile-time interface check: MongoClient must satisfy server.Shutdownable.
var _ server.Shutdownable = (*MongoClient)(nil)

// Integration tests for NewMongoClient, HealthCheck, and Close require a
// real MongoDB instance. They are tested via docker-compose in CI:
//   docker-compose up -d mongodb
//   ZEE6DO_MONGODB_URI=mongodb://localhost:27017 go test -tags=integration ./internal/database/...
