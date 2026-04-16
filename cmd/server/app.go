package main

import (
	"context"
	"fmt"

	"github.com/DeepakDP5/zee6do-server/internal/database"
	grpcserver "github.com/DeepakDP5/zee6do-server/internal/grpc"
	"github.com/DeepakDP5/zee6do-server/internal/server"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/logging"
	"github.com/DeepakDP5/zee6do-server/pkg/middleware"
	"go.uber.org/zap"
)

// App holds all the components of the application, wired together by Wire.
type App struct {
	Config          *config.Config
	Logger          *zap.Logger
	MongoClient     *database.MongoClient
	HealthChecker   *server.HealthChecker
	ShutdownManager *server.ShutdownManager
	GRPCServer      *grpcserver.Server
}

// newApp creates the App. This is called by the Wire-generated injector.
func newApp(
	cfg *config.Config,
	logger *zap.Logger,
	mongoClient *database.MongoClient,
	healthChecker *server.HealthChecker,
	grpcServer *grpcserver.Server,
) *App {
	shutdownMgr := server.NewShutdownManager(
		logger,
		healthChecker,
		cfg.Shutdown.DrainInterval,
		cfg.Shutdown.GracePeriod,
	)

	// Register components for shutdown in reverse-init order.
	// gRPC server stops first (stop accepting new RPCs), then DB.
	shutdownMgr.Register("grpc", grpcServer)
	shutdownMgr.Register("mongodb", mongoClient)

	return &App{
		Config:          cfg,
		Logger:          logger,
		MongoClient:     mongoClient,
		HealthChecker:   healthChecker,
		ShutdownManager: shutdownMgr,
		GRPCServer:      grpcServer,
	}
}

// provideLogger creates the application logger from config.
func provideLogger(cfg *config.Config) *zap.Logger {
	if cfg.Server.Environment == "development" {
		return logging.NewDevelopmentLogger()
	}
	return logging.NewLogger(cfg.Logging.Level)
}

// provideAuthConfig builds the auth interceptor configuration.
// The JWTValidator is a placeholder that accepts any token until the
// auth module provides a real implementation.
func provideAuthConfig() middleware.AuthConfig {
	return middleware.AuthConfig{
		Validator: &placeholderValidator{},
		SkipMethods: map[string]bool{
			"/zee6do.v1.AuthService/SendOTP":     true,
			"/zee6do.v1.AuthService/VerifyOTP":    true,
			"/zee6do.v1.AuthService/SocialLogin":  true,
			"/zee6do.v1.AuthService/RefreshToken": true,
			"/zee6do.v1.FlagService/GetFlags":     true,
		},
	}
}

// placeholderValidator is a temporary JWT validator used during bootstrap
// before the auth module is implemented. It accepts any non-empty token
// and returns "placeholder-user" as the user ID.
type placeholderValidator struct{}

func (p *placeholderValidator) ValidateToken(_ context.Context, token string) (string, error) {
	// Placeholder: accept any non-empty token.
	// The auth module (Task 3) will replace this with real JWT validation.
	if token == "" {
		return "", fmt.Errorf("empty token")
	}
	return "placeholder-user", nil
}
