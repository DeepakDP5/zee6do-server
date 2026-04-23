package main

import (
	zee6dov1 "github.com/DeepakDP5/zee6do-server/gen/zee6do/v1"
	"github.com/DeepakDP5/zee6do-server/internal/auth"
	"github.com/DeepakDP5/zee6do-server/internal/database"
	grpcserver "github.com/DeepakDP5/zee6do-server/internal/grpc"
	"github.com/DeepakDP5/zee6do-server/internal/server"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/DeepakDP5/zee6do-server/pkg/crypto"
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
	AuthService     *auth.Handler
}

// newApp creates the App. This is called by the Wire-generated injector.
func newApp(
	cfg *config.Config,
	logger *zap.Logger,
	mongoClient *database.MongoClient,
	healthChecker *server.HealthChecker,
	grpcServer *grpcserver.Server,
	authHandler *auth.Handler,
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

	// Register auth service on the gRPC server.
	zee6dov1.RegisterAuthServiceServer(grpcServer.GRPCServer(), authHandler)

	return &App{
		Config:          cfg,
		Logger:          logger,
		MongoClient:     mongoClient,
		HealthChecker:   healthChecker,
		ShutdownManager: shutdownMgr,
		GRPCServer:      grpcServer,
		AuthService:     authHandler,
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
//
// The real JWT validator from the crypto package is always used. In
// development it still validates signatures, but with a dev-generated secret
// (callers can mint tokens locally). In staging/production the configured
// secret is required (enforced by config.validate).
func provideAuthConfig(jwtSvc *crypto.JWTService) middleware.AuthConfig {
	return middleware.AuthConfig{
		Validator: jwtSvc,
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


