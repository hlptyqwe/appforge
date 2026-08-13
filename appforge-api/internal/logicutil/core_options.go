package logicutil

import (
	"appforge/admin-api/internal/types"
	"appforge/proto/common"
	"appforge/proto/system"
)

func CoreOptions() []types.OptionsGroup {
	options := make([]types.OptionsGroup, 0)
	options = append(options, CommonOptions()...)
	options = append(options, SystemOptions()...)
	return options
}

func CommonOptions() []types.OptionsGroup {
	return []types.OptionsGroup{
		EnumGroup("yesNo", "是否", common.YesNo_YES_NO_UNKNOWN.Descriptor()),
		EnumGroup("visible", "显示状态", common.Switch_SWITCH_UNKNOWN.Descriptor()),
		EnumGroup("enabled", "启用状态", common.Enable_ENABLE_UNKNOWN.Descriptor()),
		EnumGroup("commonStatus", "通用状态", common.Enable_ENABLE_UNKNOWN.Descriptor()),
		EnumGroup("enableStatus", "启用状态", common.Enable_ENABLE_UNKNOWN.Descriptor()),
	}
}

func SystemOptions() []types.OptionsGroup {
	return []types.OptionsGroup{
		EnumGroup("sysConfigType", "系统配置类型", system.SysConfigType_UNKNOWN.Descriptor()),
		EnumGroup("menuType", "菜单类型", system.MenuType_MENU_TYPE_UNKNOWN.Descriptor()),
		EnumGroup("method", "请求方法", system.RequestMethod_REQUEST_METHOD_UNKNOWN.Descriptor()),
		EnumGroup("applicationScope", "应用范围", system.ApplicationScope_APPLICATION_SCOPE_UNKNOWN.Descriptor()),
		EnumGroup("userType", "用户类型", system.UserType_USER_TYPE_UNKNOWN.Descriptor()),
		EnumGroup("tenantDomainStatus", "租户域名状态", system.TenantDomainStatus_TENANT_DOMAIN_STATUS_UNKNOWN.Descriptor()),
	}
}
