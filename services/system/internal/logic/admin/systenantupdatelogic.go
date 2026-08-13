package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysTenantUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantUpdateLogic {
	return &SysTenantUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantUpdateLogic) SysTenantUpdate(in *system.SysTenantUpdateReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if tenantID(l.ctx) > 0 {
		return nil, status.Error(codes.PermissionDenied, "only system administrators can manage tenants")
	}
	item, err := l.svcCtx.TenantMode.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "tenant")
	}
	if in.GetTenantName() != "" {
		item.TenantName = strings.TrimSpace(in.GetTenantName())
	}
	if in.GetEnabled() != 0 {
		item.Enabled = int64(in.GetEnabled())
	}
	if in.GetExpireTime() != 0 {
		item.ExpireTime = in.GetExpireTime()
	}
	if in.GetContactName() != "" {
		item.ContactName = nullText(in.GetContactName())
	}
	if in.GetContactPhone() != "" {
		item.ContactPhone = nullText(in.GetContactPhone())
	}
	if in.GetRemark() != "" {
		item.Remark = nullText(in.GetRemark())
	}
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.TenantMode.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update tenant failed: %v", err)
	}
	if in.GetTenantPassword() != "" {
		var owner models.SysUser
		if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &owner, "SELECT id, tenant_id, app_scope, user_type, is_owner, username, password, nickname, avatar, enabled, google_secret, google_enabled, perms_ver, last_login_ip, last_login_at, create_by, create_times, update_times FROM sys_user WHERE tenant_id = ? AND is_owner = 1 ORDER BY id LIMIT 1", item.Id); err != nil {
			return nil, status.Errorf(codes.Internal, "find tenant owner failed: %v", err)
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(in.GetTenantPassword()), bcrypt.DefaultCost)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "hash tenant password failed: %v", err)
		}
		owner.Password = string(hashed)
		owner.UpdateTimes = time.Now().UnixMilli()
		if err := l.svcCtx.UserModel.Update(l.ctx, &owner); err != nil {
			return nil, status.Errorf(codes.Internal, "update tenant owner failed: %v", err)
		}
	}
	return &system.RespBase{Base: responseBase()}, nil
}
