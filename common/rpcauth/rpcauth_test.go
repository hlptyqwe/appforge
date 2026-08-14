package rpcauth

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestValidateToken(t *testing.T) {
	if ValidateToken("short") == nil {
		t.Fatal("short internal RPC token must be rejected")
	}
	if err := ValidateToken("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("valid internal RPC token rejected: %v", err)
	}
}

func TestUnaryServerInterceptor(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"
	interceptor := UnaryServerInterceptor(token)
	handler := func(context.Context, any) (any, error) { return "ok", nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/core.Core/GetApplication"}

	if _, err := interceptor(context.Background(), nil, info, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing token status = %v, want Unauthenticated", status.Code(err))
	}
	bad := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataKey, token+"x"))
	if _, err := interceptor(bad, nil, info, handler); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("bad token status = %v, want Unauthenticated", status.Code(err))
	}
	good := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataKey, token))
	response, err := interceptor(good, nil, info, handler)
	if err != nil || response != "ok" {
		t.Fatalf("valid token response=%v err=%v", response, err)
	}
}
