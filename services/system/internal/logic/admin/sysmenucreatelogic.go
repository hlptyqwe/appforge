package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysMenuCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysMenuCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysMenuCreateLogic {
	return &SysMenuCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysMenuCreateLogic) SysMenuCreate(in *system.SysMenuCreateReq) (*system.RespBase, error) {
	if in == nil || strings.TrimSpace(in.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if in.GetParentId() < 0 {
		return nil, status.Error(codes.InvalidArgument, "parent_id is invalid")
	}
	if in.GetMenuType() == system.MenuType_MENU_TYPE_UNKNOWN {
		return nil, status.Error(codes.InvalidArgument, "menu_type is required")
	}
	appScope := effectiveAppScope(l.ctx, in.GetAppScope())
	if in.GetParentId() > 0 {
		parent, err := l.svcCtx.MenuModel.FindOne(l.ctx, in.GetParentId())
		if err != nil || parent.AppScope != appScope {
			return nil, status.Error(codes.InvalidArgument, "parent menu is invalid")
		}
	}
	now := time.Now().UnixMilli()
	item := &models.SysMenu{
		ParentId: in.GetParentId(), AppScope: appScope, Name: strings.TrimSpace(in.GetName()),
		MenuType: int64(in.GetMenuType()), Method: methodValue(in.GetMethod()), Path: strings.TrimSpace(in.GetPath()),
		Component: strings.TrimSpace(in.GetComponent()), Perms: strings.TrimSpace(in.GetPerms()), Icon: strings.TrimSpace(in.GetIcon()),
		Sort: in.GetSort(), Visible: int64(in.GetVisible()), Enabled: int64(in.GetEnabled()), CreateTimes: now, UpdateTimes: now,
	}
	if item.Visible == 0 {
		item.Visible = 1
	}
	if item.Enabled == 0 {
		item.Enabled = 1
	}
	if _, err := l.svcCtx.MenuModel.Insert(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "create menu failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
