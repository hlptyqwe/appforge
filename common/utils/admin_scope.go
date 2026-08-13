package utils

import (
	"context"
)

// ResolveAdminTenantReadScopeFromMd resolves the effective tenant filter for an
// admin read request. System administrators may omit tenantId to query all
// tenants. Tenant owners and tenant administrators are always scoped to their
// profile tenant; an explicit cross-tenant request is rejected.
func ResolveAdminTenantReadScopeFromMd(ctx context.Context, requestedTenantId int64) (int64, bool, bool, error) {
	userType, err := GetUserTypeFromMd(ctx)
	if err != nil {
		return 0, false, false, err
	}

	switch userType {
	case SysUserTypeSystemAdmin:
		return requestedTenantId, true, false, nil
	case SysUserTypeTenantOwner, SysUserTypeTenantAdmin:
		tenantId, err := GetTenantIdFromMd(ctx)
		if err != nil {
			return 0, false, false, err
		}
		if tenantId <= 0 || (requestedTenantId > 0 && requestedTenantId != tenantId) {
			return 0, false, false, nil
		}
		return tenantId, true, false, nil
	default:
		return 0, false, true, nil
	}
}

func ResolveAdminTenantWriteScopeFromMd(ctx context.Context, currentTenantId int64) (bool, bool, bool, error) {
	userType, err := GetUserTypeFromMd(ctx)
	if err != nil {
		return false, false, false, err
	}

	switch userType {
	case SysUserTypeSystemAdmin:
		return true, true, false, nil
	case SysUserTypeTenantOwner, SysUserTypeTenantAdmin:
		tenantId, err := GetTenantIdFromMd(ctx)
		if err != nil {
			return false, false, false, err
		}
		return false, currentTenantId == tenantId, false, nil
	default:
		return false, false, true, nil
	}
}
