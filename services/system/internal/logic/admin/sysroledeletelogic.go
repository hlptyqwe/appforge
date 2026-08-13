package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysRoleDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysRoleDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysRoleDeleteLogic {
	return &SysRoleDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysRoleDeleteLogic) SysRoleDelete(in *system.SysRoleDeleteReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if _, err := l.svcCtx.RoleModel.FindOne(l.ctx, in.GetId()); err != nil {
		return nil, notFound(err, "role")
	}
	if _, err := l.svcCtx.DB.ExecCtx(l.ctx, "DELETE FROM sys_user_role WHERE role_id = ?", in.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete role users failed: %v", err)
	}
	if err := l.svcCtx.RoleMenuModel.DeleteByRoleId(l.ctx, in.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete role menus failed: %v", err)
	}
	if err := l.svcCtx.RoleModel.Delete(l.ctx, in.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete role failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
