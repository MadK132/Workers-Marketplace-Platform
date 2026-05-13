package grpcmiddleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Auth(sharedSecret string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if sharedSecret == "" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok || !metadataContains(md, "x-gateway-secret", sharedSecret) {
			return nil, status.Error(codes.Unauthenticated, "invalid gateway secret")
		}

		return handler(ctx, req)
	}
}

func metadataContains(md metadata.MD, key string, expected string) bool {
	for _, value := range md.Get(key) {
		if value == expected {
			return true
		}
	}
	return false
}
