package adminlogic

import (
	"context"
	"strings"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysRoleListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysRoleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysRoleListLogic {
	return &SysRoleListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysRoleListLogic) SysRoleList(in *system.SysRoleListReq) (*system.SysRoleListResp, error) {
	if in == nil {
		in = &system.SysRoleListReq{}
	}
	tenant, err := effectiveTenant(l.ctx, 0)
	if err != nil {
		return nil, err
	}
	appScope := effectiveAppScope(l.ctx, in.GetAppScope())
	cursor, limit := pageValues(in.GetPage())
	items, total, err := l.svcCtx.RoleModel.FindPage(l.ctx, models.RolePageFilter{
		Keyword: strings.TrimSpace(in.GetKeyword()), TenantId: tenant,
		Enabled: int64(in.GetEnabled()), AppScope: appScope,
	}, cursor, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list roles failed: %v", err)
	}
	data := make([]*system.SysRoleItem, 0, len(items))
	for _, item := range items {
		data = append(data, roleItem(item))
	}
	var nextCursor int64
	if len(items) > 0 && int64(len(items)) == limit {
		nextCursor = items[len(items)-1].Id
	}
	return &system.SysRoleListResp{
		Base: responsePage(total, nextCursor > 0, nextCursor, cursor > 0, 0), Data: data,
	}, nil
}
