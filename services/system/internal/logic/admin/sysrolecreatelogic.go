package adminlogic

import (
	"context"
	"strings"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysRoleCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysRoleCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysRoleCreateLogic {
	return &SysRoleCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysRoleCreateLogic) SysRoleCreate(in *system.SysRoleCreateReq) (*system.RespBase, error) {
	if in == nil || strings.TrimSpace(in.GetName()) == "" || strings.TrimSpace(in.GetCode()) == "" {
		return nil, status.Error(codes.InvalidArgument, "name and code are required")
	}
	tenant, err := effectiveTenant(l.ctx, in.GetTenantId())
	if err != nil {
		return nil, err
	}
	appScope := effectiveAppScope(in.GetAppScope())
	if existing, err := l.svcCtx.RoleModel.FindOneByTenantIdAppScopeCode(l.ctx, tenant, appScope, strings.TrimSpace(in.GetCode())); err == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "role code already exists")
	}
	now := time.Now().UnixMilli()
	item := &models.SysRole{TenantId: tenant, AppScope: appScope, Name: strings.TrimSpace(in.GetName()), Code: strings.TrimSpace(in.GetCode()), Enabled: int64(in.GetEnabled()), Remark: strings.TrimSpace(in.GetRemark()), CreateTimes: now, UpdateTimes: now}
	if item.Enabled == 0 {
		item.Enabled = 1
	}
	if _, err := l.svcCtx.RoleModel.Insert(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "create role failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
