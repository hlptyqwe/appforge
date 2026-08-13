package utils

import (
	"context"
	"strconv"
	"testing"

	"google.golang.org/grpc/metadata"
)

func adminScopeContext(userType, tenantId int64) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		CtxKeyUserType, strconv.FormatInt(userType, 10),
		CtxKeyTenantId, strconv.FormatInt(tenantId, 10),
	))
}

func TestResolveAdminTenantReadScopeFromMd(t *testing.T) {
	tests := []struct {
		name       string
		userType   int64
		profile    int64
		requested  int64
		wantTenant int64
		wantAllow  bool
		wantForbid bool
	}{
		{name: "system admin all tenants", userType: SysUserTypeSystemAdmin, requested: 0, wantAllow: true},
		{name: "system admin selected tenant", userType: SysUserTypeSystemAdmin, requested: 42, wantTenant: 42, wantAllow: true},
		{name: "tenant owner implicit own tenant", userType: SysUserTypeTenantOwner, profile: 42, requested: 0, wantTenant: 42, wantAllow: true},
		{name: "tenant admin own tenant", userType: SysUserTypeTenantAdmin, profile: 42, requested: 42, wantTenant: 42, wantAllow: true},
		{name: "tenant admin cross tenant", userType: SysUserTypeTenantAdmin, profile: 42, requested: 43},
		{name: "unknown admin type", userType: 99, profile: 42, requested: 42, wantForbid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenantId, allowed, forbidden, err := ResolveAdminTenantReadScopeFromMd(
				adminScopeContext(tt.userType, tt.profile), tt.requested,
			)
			if err != nil {
				t.Fatalf("ResolveAdminTenantReadScopeFromMd() error = %v", err)
			}
			if tenantId != tt.wantTenant || allowed != tt.wantAllow || forbidden != tt.wantForbid {
				t.Fatalf("got (%d, %v, %v), want (%d, %v, %v)",
					tenantId, allowed, forbidden, tt.wantTenant, tt.wantAllow, tt.wantForbid)
			}
		})
	}
}
