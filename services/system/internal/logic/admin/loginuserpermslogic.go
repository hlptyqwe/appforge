package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LoginUserPermsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginUserPermsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginUserPermsLogic {
	return &LoginUserPermsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginUserPermsLogic) LoginUserPerms(in *system.LoginUserPermsReq) (*system.LoginUserPermsResp, error) {
	if in == nil || in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	var perms []string
	// The role-menu relation is intentionally joined through its own table;
	// keeping this query explicit prevents a user from choosing arbitrary role IDs.
	query := "SELECT DISTINCT m.perms FROM sys_user_role ur JOIN sys_role_menu rm ON rm.role_id = ur.role_id JOIN sys_menu m ON m.id = rm.menu_id WHERE ur.user_id = ? AND m.enabled = 1 AND m.perms <> '' ORDER BY m.perms"
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &perms, query, in.UserId); err != nil {
		return nil, status.Errorf(codes.Internal, "query user permissions failed: %v", err)
	}

	return &system.LoginUserPermsResp{Perms: perms}, nil
}
