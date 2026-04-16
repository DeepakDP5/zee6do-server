package main

import (
	"github.com/DeepakDP5/zee6do-server/internal/database"
	"github.com/DeepakDP5/zee6do-server/internal/server"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/logging"
	"go.uber.org/zap"
)

// App holds all the components of the application, wired together by Wire.
type App struct {
	Config          *config.Config
	Logger          *zap.Logger
	MongoClient     *database.MongoClient
	HealthChecker   *server.HealthChecker
	ShutdownManager *server.ShutdownManager
}

// newApp creates the App. This is called by the Wire-generated injector.
func newApp(
	cfg *config.Config,
	logger *zap.Logger,
	mongoClient *database.MongoClient,
	healthChecker *server.HealthChecker,
) *App {
	shutdownMgr := server.NewShutdownManager(
		logger,
		healthChecker,
		cfg.Shutdown.DrainInterval,
		cfg.Shutdown.GracePeriod,
	)

	// Register components for shutdown in reverse-init order
	shutdownMgr.Register("mongodb", mongoClient)

	return &App{
		Config:          cfg,
		Logger:          logger,
		MongoClient:     mongoClient,
		HealthChecker:   healthChecker,
		ShutdownManager: shutdownMgr,
	}
}

// provideLogger creates the application logger from config.
func provideLogger(cfg *config.Config) *zap.Logger {
	if cfg.Server.Environment == "development" {
		return logging.NewDevelopmentLogger()
	}
	return logging.NewLogger(cfg.Logging.Level)
}
