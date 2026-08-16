package worker

import (
	"errors"
	"testing"

	"appforge/services/builder/internal/config"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateRemoteSigningCapability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		capability string
		localRoot  string
		wantErr    bool
	}{
		{name: "not declared", capability: `{"apk":true}`},
		{name: "explicitly disabled", capability: `{"remoteSigning":false}`},
		{name: "enabled with provider", capability: `{"remoteSigning":true}`, localRoot: "/run/secrets"},
		{name: "enabled without provider", capability: `{"remoteSigning":true}`, wantErr: true},
		{name: "non boolean", capability: `{"remoteSigning":"true"}`, localRoot: "/run/secrets", wantErr: true},
		{name: "invalid object", capability: `[]`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var cfg config.Config
			cfg.Builder.CapabilityJson = tt.capability
			cfg.SecretProviders.LocalRoot = tt.localRoot
			if err := validateRemoteSigningCapability(cfg); (err != nil) != tt.wantErr {
				t.Fatalf("validateRemoteSigningCapability() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestIsDefinitiveOwnershipError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "not found", err: status.Error(codes.NotFound, "fenced"), want: true},
		{name: "failed precondition", err: status.Error(codes.FailedPrecondition, "lease expired"), want: true},
		{name: "permission denied", err: status.Error(codes.PermissionDenied, "wrong owner"), want: true},
		{name: "unavailable is ambiguous", err: status.Error(codes.Unavailable, "network"), want: false},
		{name: "plain error is ambiguous", err: errors.New("transport closed"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isDefinitiveOwnershipError(tt.err); got != tt.want {
				t.Fatalf("isDefinitiveOwnershipError() = %t, want %t", got, tt.want)
			}
		})
	}
}
