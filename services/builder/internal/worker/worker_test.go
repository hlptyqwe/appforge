package worker

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
