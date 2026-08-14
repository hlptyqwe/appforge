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

type SysUserListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysUserListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysUserListLogic {
	return &SysUserListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysUserListLogic) SysUserList(in *system.SysUserListReq) (*system.SysUserListResp, error) {
	if in == nil {
		in = &system.SysUserListReq{}
	}
	tenant, err := effectiveTenant(l.ctx, 0)
	if err != nil {
		return nil, err
	}
	appScope := effectiveAppScope(l.ctx, in.GetAppScope())
	cursor, limit := pageValues(in.GetPage())
	items, total, err := l.svcCtx.UserModel.FindPage(l.ctx, models.UserPageFilter{
		Keyword: strings.TrimSpace(in.GetKeyword()), TenantId: tenant,
		Enabled: int64(in.GetEnabled()), AppScope: appScope,
	}, cursor, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list users failed: %v", err)
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id)
	}
	roleMap, err := l.svcCtx.UserRoleModel.FindRoleIdsByUserIds(l.ctx, ids)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query user roles failed: %v", err)
	}
	data := make([]*system.SysUserItem, 0, len(items))
	for _, item := range items {
		data = append(data, userItem(item, roleMap[item.Id]))
	}
	var nextCursor int64
	if len(items) > 0 && int64(len(items)) == limit {
		nextCursor = items[len(items)-1].Id
	}
	return &system.SysUserListResp{
		Base: responsePage(total, nextCursor > 0, nextCursor, cursor > 0, 0), Data: data,
	}, nil
}
