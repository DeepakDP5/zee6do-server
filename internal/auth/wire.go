package auth

import (
	"github.com/DeepakDP5/zee6do-server/pkg/crypto"
	"github.com/DeepakDP5/zee6do-server/pkg/middleware"
	"github.com/google/wire"
)

// ProviderSet is the Wire provider set for the auth module.
//
// It registers the Mongo repository, the JWT service (also bound to the
// middleware.JWTValidator interface used by the gRPC auth interceptor),
// the business service, and the gRPC handler.
var ProviderSet = wire.NewSet(
	NewMongoRepository,
	wire.Bind(new(Repository), new(*MongoRepository)),

	crypto.NewJWTService,
	wire.Bind(new(middleware.JWTValidator), new(*crypto.JWTService)),

	NewService,
	NewHandler,
)
