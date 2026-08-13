package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysMenuDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysMenuDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysMenuDeleteLogic {
	return &SysMenuDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysMenuDeleteLogic) SysMenuDelete(in *system.SysMenuDeleteReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if _, err := l.svcCtx.MenuModel.FindOne(l.ctx, in.GetId()); err != nil {
		return nil, notFound(err, "menu")
	}
	var children int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &children, "SELECT COUNT(1) FROM sys_menu WHERE parent_id = ?", in.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "check menu children failed: %v", err)
	}
	if children > 0 {
		return nil, status.Error(codes.FailedPrecondition, "menu has children")
	}
	if _, err := l.svcCtx.DB.ExecCtx(l.ctx, "DELETE FROM sys_role_menu WHERE menu_id = ?", in.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete menu permissions failed: %v", err)
	}
	if err := l.svcCtx.MenuModel.Delete(l.ctx, in.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete menu failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
