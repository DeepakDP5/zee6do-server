package users

import (
	"github.com/DeepakDP5/zee6do-server/internal/database"
	"github.com/google/wire"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ProviderSet is the Wire provider set for the users module.
var ProviderSet = wire.NewSet(
	provideDatabase,
	NewMongoRepository,
	wire.Bind(new(Repository), new(*MongoRepository)),
)

// provideDatabase extracts the *mongo.Database from MongoClient so it can be
// injected into repositories by Wire.
func provideDatabase(client *database.MongoClient) *mongo.Database {
	return client.Database()
}
