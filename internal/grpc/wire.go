package grpc

import "github.com/google/wire"

// ProviderSet is the Wire provider set for the gRPC server package.
var ProviderSet = wire.NewSet(
	NewServer,
)
