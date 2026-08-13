package systemlogic

import (
	"context"
	"strings"

	"appforge/common/utils"
	"appforge/proto/common"
	"appforge/proto/system"
	"appforge/services/system/models"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func responseBase() *common.RespBase {
	return &common.RespBase{Code: 200, Msg: "OK"}
}

func tenantID(ctx context.Context) int64 {
	if value, err := utils.GetTrustedTenantIdFromCtx(ctx); err == nil {
		return value
	}
	if value, err := utils.GetTenantIdFromMd(ctx); err == nil {
		return value
	}
	return 0
}

func pageValues(page *common.PageReq) (int64, int64) {
	if page == nil {
		return 0, 20
	}
	limit := page.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page.Cursor, limit
}

func responsePage(total int64, hasNext bool, nextCursor int64) *common.RespBase {
	return &common.RespBase{Code: 200, Msg: "OK", Total: total, HasNext: hasNext, NextCursor: nextCursor}
}

func configItem(item *models.SysConfig) *system.SysConfigItem {
	if item == nil {
		return nil
	}
	return &system.SysConfigItem{Id: item.Id, TenantId: item.TenantId, ConfigKey: item.ConfigKey.String, ConfigValue: item.ConfigValue.String, Remark: item.Remark.String, CreateTimes: item.CreateTimes, UpdateTimes: item.UpdateTimes}
}

func tenantItem(item *models.SysTenant) *system.SysTenantItem {
	if item == nil {
		return nil
	}
	return &system.SysTenantItem{Id: item.Id, TenantCode: item.TenantCode, TenantName: item.TenantName, Enabled: common.Enable(item.Enabled), ExpireTime: item.ExpireTime, ContactName: item.ContactName.String, ContactPhone: item.ContactPhone.String, Remark: item.Remark.String, CreateTimes: item.CreateTimes, UpdateTimes: item.UpdateTimes, LoginIp: item.LoginIp.String, LoginTime: item.LoginTime, LoginCount: item.LoginCount}
}

func requiredConfigTenant(ctx context.Context, requested *int64) (int64, error) {
	current := tenantID(ctx)
	if requested != nil {
		if current > 0 && *requested != current {
			return 0, status.Error(codes.PermissionDenied, "cross-tenant access is not allowed")
		}
		return *requested, nil
	}
	return current, nil
}

func trim(value string) string { return strings.TrimSpace(value) }
