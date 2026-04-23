//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/DeepakDP5/zee6do-server/internal/auth"
	"github.com/DeepakDP5/zee6do-server/internal/database"
	grpcserver "github.com/DeepakDP5/zee6do-server/internal/grpc"
	"github.com/DeepakDP5/zee6do-server/internal/server"
	"github.com/DeepakDP5/zee6do-server/internal/users"
	"github.com/DeepakDP5/zee6do-server/pkg/config"
	"github.com/google/wire"
)

// InitializeApp wires all dependencies and returns the fully assembled App.
// Wire generates the implementation in wire_gen.go.
func InitializeApp(ctx context.Context, cfg *config.Config) (*App, error) {
	wire.Build(
		provideLogger,
		provideAuthConfig,
		database.ProviderSet,
		users.ProviderSet,
		auth.ProviderSet,
		server.ProviderSet,
		grpcserver.ProviderSet,
		newApp,
	)
	return nil, nil
}
