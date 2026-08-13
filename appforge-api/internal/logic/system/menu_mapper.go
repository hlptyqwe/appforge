package system

import (
	"appforge/admin-api/internal/types"
	"appforge/proto/system"
)

func mapSysMenuItem(item *system.SysMenuItem) types.SysMenuItem {
	if item == nil {
		return types.SysMenuItem{}
	}

	return types.SysMenuItem{
		Id:        item.Id,
		ParentId:  item.ParentId,
		Name:      item.Name,
		MenuType:  fromMenuType(item.MenuType),
		Method:    fromRequestMethod(item.Method),
		Path:      item.Path,
		Component: item.Component,
		Icon:      item.Icon,
		Sort:      item.Sort,
		Visible:   fromVisibleStatus(item.Visible),
		Enabled:   fromCommonStatus(item.Enabled),
		Perms:     item.Perms,
		AppScope:  int64(item.AppScope),
	}
}
