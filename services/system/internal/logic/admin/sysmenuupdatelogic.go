package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysMenuUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysMenuUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysMenuUpdateLogic {
	return &SysMenuUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysMenuUpdateLogic) SysMenuUpdate(in *system.SysMenuUpdateReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 || strings.TrimSpace(in.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "id and name are required")
	}
	item, err := l.svcCtx.MenuModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "menu")
	}
	if in.GetParentId() == item.Id {
		return nil, status.Error(codes.InvalidArgument, "menu cannot be its own parent")
	}
	item.ParentId = in.GetParentId()
	item.Name = strings.TrimSpace(in.GetName())
	if in.GetMenuType() != system.MenuType_MENU_TYPE_UNKNOWN {
		item.MenuType = int64(in.GetMenuType())
	}
	if in.GetMethod() != system.RequestMethod_REQUEST_METHOD_UNKNOWN {
		item.Method = methodValue(in.GetMethod())
	}
	item.Path = strings.TrimSpace(in.GetPath())
	item.Component = strings.TrimSpace(in.GetComponent())
	item.Icon = strings.TrimSpace(in.GetIcon())
	item.Perms = strings.TrimSpace(in.GetPerms())
	if in.GetSort() != 0 {
		item.Sort = in.GetSort()
	}
	if in.GetVisible() != 0 {
		item.Visible = int64(in.GetVisible())
	}
	if in.GetEnabled() != 0 {
		item.Enabled = int64(in.GetEnabled())
	}
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.MenuModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update menu failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
