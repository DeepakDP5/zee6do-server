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
// In development, a placeholder validator accepts any token. In staging/production
// the placeholder is rejected at startup to fail closed — a real validator must be
// wired before deploying outside development.
func provideAuthConfig(cfg *config.Config) middleware.AuthConfig {
	var validator middleware.JWTValidator
	if cfg.Server.Environment == "development" {
		validator = &placeholderValidator{}
	} else {
		// Fail closed: no real validator means no authenticated RPCs can succeed.
		// This will be replaced when the auth module (Task 3) is implemented.
		validator = &rejectAllValidator{}
	}

	return middleware.AuthConfig{
		Validator: validator,
		SkipMethods: map[string]bool{
			// gRPC health protocol — used by load balancers, Kubernetes probes, and
			// monitoring systems. These must never require authentication.
			"/grpc.health.v1.Health/Check": true,
			"/grpc.health.v1.Health/Watch": true,
			"/grpc.health.v1.Health/List":  true,

			// Application auth endpoints — unauthenticated by design.
			"/zee6do.v1.AuthService/SendOTP":     true,
			"/zee6do.v1.AuthService/VerifyOTP":    true,
			"/zee6do.v1.AuthService/SocialLogin":  true,
			"/zee6do.v1.AuthService/RefreshToken": true,
			"/zee6do.v1.FlagService/GetFlags":     true,
		},
	}
}

// rejectAllValidator rejects all tokens. Used in staging/production when no
// real JWT validator has been wired yet, ensuring the system fails closed.
type rejectAllValidator struct{}

func (r *rejectAllValidator) ValidateToken(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("no JWT validator configured for this environment")
}

// placeholderValidator is a temporary JWT validator used during bootstrap
// before the auth module is implemented. It accepts any token and returns
// "placeholder-user" as the user ID.
//
// NOTE: The auth interceptor already guards against empty tokens before
// calling ValidateToken, so no empty-token check is needed here.
//
// WARNING: This validator is development-only. It must be replaced with
// real JWT validation before staging/production use. See Review item #1.
type placeholderValidator struct{}

func (p *placeholderValidator) ValidateToken(_ context.Context, _ string) (string, error) {
	return "placeholder-user", nil
}
