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

type SysUserUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysUserUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysUserUpdateLogic {
	return &SysUserUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysUserUpdateLogic) SysUserUpdate(in *system.SysUserUpdateReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	item, err := l.svcCtx.UserModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "user")
	}
	if err := requireItemAppScope(l.ctx, item.AppScope); err != nil {
		return nil, err
	}
	if _, err := effectiveTenant(l.ctx, item.TenantId); err != nil {
		return nil, err
	}
	if in.GetTenantId() > 0 && in.GetTenantId() != item.TenantId {
		return nil, status.Error(codes.PermissionDenied, "user tenant cannot be changed")
	}
	if in.GetNickname() != "" {
		item.Nickname = strings.TrimSpace(in.GetNickname())
	}
	if in.GetAvatar() != "" {
		item.Avatar = strings.TrimSpace(in.GetAvatar())
	}
	if in.GetEnabled() != 0 {
		item.Enabled = int64(in.GetEnabled())
	}
	if in.GetUserType() != system.UserType_USER_TYPE_UNKNOWN {
		item.UserType = int64(in.GetUserType())
	}
	if in.GetIsOwner() != 0 {
		item.IsOwner = int64(in.GetIsOwner())
	}
	item.PermsVer++
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.UserModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update user failed: %v", err)
	}
	if len(in.GetRoleIds()) > 0 || in.RoleIds != nil {
		if err := replaceUserRoles(l.ctx, l.svcCtx, item, in.GetRoleIds()); err != nil {
			return nil, err
		}
	}
	return &system.RespBase{Base: responseBase()}, nil
}
