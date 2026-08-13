package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysUserDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysUserDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysUserDeleteLogic {
	return &SysUserDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysUserDeleteLogic) SysUserDelete(in *system.SysUserDeleteReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if in.GetId() == actorID(l.ctx) {
		return nil, status.Error(codes.FailedPrecondition, "cannot delete current user")
	}
	item, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if _, err := effectiveTenant(l.ctx, item.TenantId); err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.DB.ExecCtx(l.ctx, "DELETE FROM sys_user_role WHERE user_id = ?", item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete user roles failed: %v", err)
	}
	if err := l.svcCtx.UserModel.Delete(l.ctx, item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete user failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
