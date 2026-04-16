// Package main is the entry point for the zee6do server.
// It loads configuration, initializes all dependencies via Wire,
// runs database migrations, and manages the application lifecycle
// including graceful shutdown on OS signals.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/DeepakDP5/zee6do-server/internal/database"
	"github.com/DeepakDP5/zee6do-server/migrations"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"go.uber.org/zap"
)

func main() {
	configPath := flag.String("config", "", "path to config YAML file (optional)")
	flag.Parse()

	// Load configuration
	var cfg *config.Config
	if *configPath != "" {
		cfg = config.Load(*configPath)
	} else {
		cfg = config.LoadFromEnv()
	}

	// Initialize application via Wire
	ctx := context.Background()
	app, err := InitializeApp(ctx, cfg)
	if err != nil {
		// Use a temporary logger since the app logger isn't available yet
		tempLogger, logErr := zap.NewProduction()
		if logErr != nil {
			tempLogger = zap.NewNop()
		}
		tempLogger.Fatal("failed to initialize application", zap.Error(err))
	}
	defer app.Logger.Sync() //nolint:errcheck // best-effort flush on exit

	app.Logger.Info("zee6do server starting",
		zap.String("environment", cfg.Server.Environment),
		zap.Int("grpc_port", cfg.Server.GRPCPort),
	)

	// Run database migrations
	migrationRunner := database.NewMigrationRunner(app.MongoClient.Database(), app.Logger)
	migrations.Register(migrationRunner)
	if err := migrationRunner.Run(ctx); err != nil {
		app.Logger.Fatal("failed to run migrations", zap.Error(err))
	}

	app.Logger.Info("zee6do server ready",
		zap.Int("grpc_port", cfg.Server.GRPCPort),
	)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	app.Logger.Info("received shutdown signal", zap.String("signal", sig.String()))
	app.ShutdownManager.Shutdown()
}
