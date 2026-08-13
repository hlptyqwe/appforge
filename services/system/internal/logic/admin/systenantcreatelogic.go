package adminlogic

import (
	"context"
	"fmt"
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

type SysTenantCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantCreateLogic {
	return &SysTenantCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantCreateLogic) SysTenantCreate(in *system.SysTenantCreateReq) (*system.RespBase, error) {
	if in == nil || strings.TrimSpace(in.GetUsername()) == "" || strings.TrimSpace(in.GetTenantName()) == "" || in.GetTenantPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "username, tenant_name and tenant_password are required")
	}
	if tenantID(l.ctx) > 0 {
		return nil, status.Error(codes.PermissionDenied, "only system administrators can manage tenants")
	}
	password, err := bcrypt.GenerateFromPassword([]byte(in.GetTenantPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash tenant password failed: %v", err)
	}
	now := time.Now().UnixMilli()
	code := strings.ToLower(strings.TrimSpace(in.GetUsername()))
	code = strings.NewReplacer(" ", "_", "/", "_", "\\", "_").Replace(code)
	if len(code) > 64 {
		code = code[:64]
	}
	if code == "" {
		code = fmt.Sprintf("tenant_%d", now)
	}
	tenant := &models.SysTenant{
		TenantCode: code, TenantName: strings.TrimSpace(in.GetTenantName()), Enabled: int64(in.GetEnabled()),
		ExpireTime: in.GetExpireTime(), ContactName: nullText(in.GetContactName()), ContactPhone: nullText(in.GetContactPhone()),
		Remark: nullText(in.GetRemark()), CreateBy: nullText(fmt.Sprintf("%d", actorID(l.ctx))), CreateTimes: now, UpdateTimes: now,
	}
	if tenant.Enabled == 0 {
		tenant.Enabled = 1
	}
	result, err := l.svcCtx.TenantMode.Insert(l.ctx, tenant)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create tenant failed: %v", err)
	}
	tenant.Id, err = result.LastInsertId()
	if err != nil || tenant.Id <= 0 {
		return nil, status.Errorf(codes.Internal, "resolve created tenant id failed: %v", err)
	}
	owner := &models.SysUser{
		TenantId: tenant.Id, AppScope: int64(system.ApplicationScope_APPLICATION_SCOPE_ADMIN),
		UserType: int64(system.UserType_USER_TYPE_TENANT_OWNER), IsOwner: 1,
		Username: strings.TrimSpace(in.GetUsername()), Password: string(password), Nickname: tenant.TenantName,
		Enabled: 1, GoogleEnabled: 2, PermsVer: 1, CreateBy: actorID(l.ctx), CreateTimes: now, UpdateTimes: now,
	}
	if _, err := l.svcCtx.UserModel.Insert(l.ctx, owner); err != nil {
		return nil, status.Errorf(codes.Internal, "create tenant owner failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
