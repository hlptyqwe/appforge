package utils

import (
	"context"
	"testing"
)

func TestGetTrustedTenantIdFromCtx(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		wantID int64
	}{
		{
			name: "signed legacy tenant overrides request header",
			ctx: context.WithValue(
				context.WithValue(context.Background(), CtxKeyTenantId, int64(99)),
				"expand", `{"tenantId":12}`,
			),
			wantID: 12,
		},
		{
			name:   "signed compact tenant",
			ctx:    context.WithValue(context.Background(), "expand", `{"tid":13}`),
			wantID: 13,
		},
		{
			name:   "trusted resolved public tenant",
			ctx:    context.WithValue(context.Background(), CtxKeyTenantId, int64(14)),
			wantID: 14,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTrustedTenantIdFromCtx(tt.ctx)
			if err != nil {
				t.Fatalf("GetTrustedTenantIdFromCtx() error = %v", err)
			}
			if got != tt.wantID {
				t.Fatalf("GetTrustedTenantIdFromCtx() = %d, want %d", got, tt.wantID)
			}
		})
	}
}

func TestGetTrustedTenantIdFromCtxRejectsInvalidSignedClaim(t *testing.T) {
	ctx := context.WithValue(
		context.WithValue(context.Background(), CtxKeyTenantId, int64(99)),
		"expand", `{"tenantId":0}`,
	)
	if _, err := GetTrustedTenantIdFromCtx(ctx); err == nil {
		t.Fatal("expected invalid signed tenant claim to be rejected")
	}
}
