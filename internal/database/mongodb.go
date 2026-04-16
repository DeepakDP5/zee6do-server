// Package database provides MongoDB connection management with tuned
// connection pooling, health checks, and clean shutdown.
//
// The MongoClient wraps the official MongoDB Go driver and is injected
// via Wire. Service code accesses MongoDB through repository interfaces,
// never through this package directly.
package database

import (
	"context"
	"fmt"

	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

// MongoClient wraps the MongoDB client with configuration and lifecycle methods.
type MongoClient struct {
	client   *mongo.Client
	database *mongo.Database
	logger   *zap.Logger
	cfg      config.MongoDBConfig
}

// NewMongoClient creates a new MongoDB client with tuned connection pool settings.
// It connects immediately and verifies connectivity with a ping.
func NewMongoClient(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*MongoClient, error) {
	mongoCfg := cfg.MongoDB

	opts := options.Client().
		ApplyURI(mongoCfg.URI).
		SetMaxPoolSize(mongoCfg.MaxPoolSize).
		SetConnectTimeout(mongoCfg.ConnectTimeout).
		SetTimeout(mongoCfg.Timeout).
		SetServerSelectionTimeout(mongoCfg.ServerSelectionTimeout).
		SetRetryWrites(true).
		SetRetryReads(true)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("database.NewMongoClient: connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("database.NewMongoClient: ping: %w", err)
	}

	logger.Info("connected to MongoDB",
		zap.String("database", mongoCfg.Database),
	)

	return &MongoClient{
		client:   client,
		database: client.Database(mongoCfg.Database),
		logger:   logger,
		cfg:      mongoCfg,
	}, nil
}

// Database returns the configured MongoDB database instance.
// Repository implementations use this to access collections.
func (m *MongoClient) Database() *mongo.Database {
	return m.database
}

// Client returns the underlying MongoDB client.
// Prefer Database() for most operations.
func (m *MongoClient) Client() *mongo.Client {
	return m.client
}

// HealthCheck pings MongoDB to verify connectivity.
func (m *MongoClient) HealthCheck(ctx context.Context) error {
	if err := m.client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("database.HealthCheck: %w", err)
	}
	return nil
}

// Close disconnects the MongoDB client gracefully.
func (m *MongoClient) Close(ctx context.Context) error {
	m.logger.Info("disconnecting from MongoDB")
	if err := m.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("database.Close: %w", err)
	}
	m.logger.Info("disconnected from MongoDB")
	return nil
}
