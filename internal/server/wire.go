package server

import "github.com/google/wire"

// ProviderSet is the Wire provider set for the server package.
var ProviderSet = wire.NewSet(
	NewHealthChecker,
)
