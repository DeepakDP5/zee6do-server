package database

import "github.com/google/wire"

// ProviderSet is the Wire provider set for the database package.
var ProviderSet = wire.NewSet(
	NewMongoClient,
)
