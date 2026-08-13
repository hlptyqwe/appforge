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

type SysUserCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysUserCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysUserCreateLogic {
	return &SysUserCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysUserCreateLogic) SysUserCreate(in *system.SysUserCreateReq) (*system.RespBase, error) {
	if in == nil || strings.TrimSpace(in.GetUsername()) == "" || in.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
	}
	tenant, err := effectiveTenant(l.ctx, in.GetTenantId())
	if err != nil {
		return nil, err
	}
	appScope := effectiveAppScope(in.GetAppScope())
	if existing, err := l.svcCtx.UserModel.FindOneByTenantIdAppScopeUsername(l.ctx, tenant, appScope, strings.TrimSpace(in.GetUsername())); err == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "username already exists")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(in.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash password failed: %v", err)
	}
	now := time.Now().UnixMilli()
	item := &models.SysUser{
		TenantId: tenant, AppScope: appScope, UserType: int64(in.GetUserType()), IsOwner: int64(in.GetIsOwner()),
		Username: strings.TrimSpace(in.GetUsername()), Password: string(hashed), Nickname: strings.TrimSpace(in.GetNickname()),
		Avatar: strings.TrimSpace(in.GetAvatar()), Enabled: int64(in.GetEnabled()), GoogleEnabled: 2, PermsVer: 1,
		CreateBy: actorID(l.ctx), CreateTimes: now, UpdateTimes: now,
	}
	if item.UserType == 0 {
		item.UserType = int64(system.UserType_USER_TYPE_TENANT_ADMIN)
	}
	if item.IsOwner == 0 {
		item.IsOwner = 2
	}
	if item.Enabled == 0 {
		item.Enabled = 1
	}
	if item.UserType == int64(system.UserType_USER_TYPE_SYSTEM_ADMIN) {
		item.TenantId = 0
	}
	result, err := l.svcCtx.UserModel.Insert(l.ctx, item)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create user failed: %v", err)
	}
	item.Id, _ = result.LastInsertId()
	for _, roleID := range in.GetRoleIds() {
		if roleID <= 0 {
			continue
		}
		if _, err := l.svcCtx.UserRoleModel.Insert(l.ctx, &models.SysUserRole{TenantId: item.TenantId, UserId: item.Id, RoleId: roleID}); err != nil {
			return nil, status.Errorf(codes.Internal, "assign user roles failed: %v", err)
		}
	}
	return &system.RespBase{Base: responseBase()}, nil
}
