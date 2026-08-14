package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysUserDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysUserDetailLogic {
	return &SysUserDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysUserDetailLogic) SysUserDetail(in *system.SysUserDetailReq) (*system.SysUserDetailResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	user, err := l.svcCtx.UserModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, notFound(err, "user")
	}
	if err := requireItemAppScope(l.ctx, user.AppScope); err != nil {
		return nil, err
	}

	requestUserID := actorID(l.ctx)
	if requestUserID > 0 && requestUserID != user.Id {
		requestUser, err := l.svcCtx.UserModel.FindOne(l.ctx, requestUserID)
		if err != nil {
			return nil, notFound(err, "login user")
		}
		if requestUser.TenantId > 0 && requestUser.TenantId != user.TenantId {
			return nil, status.Error(codes.PermissionDenied, "cross-tenant user access is not allowed")
		}
	}

	var roleIDs []int64
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &roleIDs,
		"SELECT role_id FROM sys_user_role WHERE user_id = ? ORDER BY role_id", user.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "query user roles failed: %v", err)
	}

	return &system.SysUserDetailResp{Base: responseBase(), Data: userItem(user, roleIDs)}, nil
}
