package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysTenantDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantDeleteLogic {
	return &SysTenantDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantDeleteLogic) SysTenantDelete(in *system.SysTenantDeleteReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if tenantID(l.ctx) > 0 {
		return nil, status.Error(codes.PermissionDenied, "only system administrators can manage tenants")
	}
	if _, err := l.svcCtx.TenantMode.FindOne(l.ctx, in.GetId()); err != nil {
		return nil, notFound(err, "tenant")
	}
	// The schema intentionally has no foreign keys, so remove dependent rows explicitly.
	queries := []string{
		"DELETE FROM sys_tenant_domain WHERE tenant_id = ?",
		"DELETE FROM sys_user_role WHERE tenant_id = ?",
		"DELETE FROM sys_role_menu WHERE tenant_id = ?",
		"DELETE FROM sys_role WHERE tenant_id = ?",
		"DELETE FROM sys_user WHERE tenant_id = ?",
	}
	for _, query := range queries {
		if _, err := l.svcCtx.DB.ExecCtx(l.ctx, query, in.GetId()); err != nil {
			return nil, status.Errorf(codes.Internal, "delete tenant dependencies failed: %v", err)
		}
	}
	if err := l.svcCtx.TenantMode.Delete(l.ctx, in.GetId()); err != nil {
		return nil, status.Errorf(codes.Internal, "delete tenant failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
