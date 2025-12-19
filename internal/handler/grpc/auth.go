package grpc

import (
	"context"
	"slices"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const apiKeyHeader = "x-api-key"

// APIKeyAuthInterceptor returns a gRPC unary interceptor that validates API keys.
// If validAPIKeys is empty, authentication is disabled (all requests pass).
func APIKeyAuthInterceptor(validAPIKeys []string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth if no API keys configured
		if len(validAPIKeys) == 0 {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		keys := md.Get(apiKeyHeader)
		if len(keys) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing API key")
		}

		apiKey := keys[0]
		if !slices.Contains(validAPIKeys, apiKey) {
			return nil, status.Error(codes.Unauthenticated, "invalid API key")
		}

		return handler(ctx, req)
	}
}
