package rpcauth

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const MetadataKey = "x-appforge-internal-token"

type Config struct {
	Token string `json:"Token" yaml:"Token"`
}

func ValidateToken(token string) error {
	if len(strings.TrimSpace(token)) < 32 {
		return errors.New("InternalRpc.Token must contain at least 32 characters")
	}
	return nil
}

func UnaryClientInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(metadata.AppendToOutgoingContext(ctx, MetadataKey, token), method, req, reply, cc, opts...)
	}
}

func UnaryServerInterceptor(expected string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod != "/grpc.health.v1.Health/Check" && !authorized(ctx, expected) {
			return nil, status.Error(codes.Unauthenticated, "internal RPC authentication failed")
		}
		return handler(ctx, req)
	}
}

func StreamServerInterceptor(expected string) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !authorized(stream.Context(), expected) {
			return status.Error(codes.Unauthenticated, "internal RPC authentication failed")
		}
		return handler(srv, stream)
	}
}

func authorized(ctx context.Context, expected string) bool {
	values := metadata.ValueFromIncomingContext(ctx, MetadataKey)
	if len(values) != 1 || len(values[0]) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(values[0]), []byte(expected)) == 1
}
