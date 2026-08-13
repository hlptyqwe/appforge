package applogic

import (
	"context"

	"appforge/common/utils"
	"appforge/proto/common"
	"appforge/proto/system"
	"appforge/services/system/models"
)

func responseBase() *common.RespBase { return &common.RespBase{Code: 200, Msg: "OK"} }

func tenantID(ctx context.Context) int64 {
	if value, err := utils.GetTrustedTenantIdFromCtx(ctx); err == nil {
		return value
	}
	if value, err := utils.GetTenantIdFromMd(ctx); err == nil {
		return value
	}
	return 0
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
