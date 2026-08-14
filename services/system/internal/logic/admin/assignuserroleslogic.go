package adminlogic

import (
	"context"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AssignUserRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignUserRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignUserRolesLogic {
	return &AssignUserRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AssignUserRolesLogic) AssignUserRoles(in *system.AssignUserRolesReq) (*system.RespBase, error) {
	if in == nil || in.GetUserId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	item, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetUserId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if err := requireItemAppScope(l.ctx, item.AppScope); err != nil {
		return nil, err
	}
	if _, err := effectiveTenant(l.ctx, item.TenantId); err != nil {
		return nil, err
	}
	if err := replaceUserRoles(l.ctx, l.svcCtx, item, in.GetRoleIds()); err != nil {
		return nil, err
	}
	item.PermsVer++
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update user permission version failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
