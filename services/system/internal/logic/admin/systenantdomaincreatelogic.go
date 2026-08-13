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

type SysTenantDomainCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantDomainCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantDomainCreateLogic {
	return &SysTenantDomainCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantDomainCreateLogic) SysTenantDomainCreate(in *system.SysTenantDomainCreateReq) (*system.RespBase, error) {
	if in == nil || in.GetTenantId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	tenant, err := effectiveTenant(l.ctx, in.GetTenantId())
	if err != nil {
		return nil, err
	}
	origin, err := normalizeOrigin(in.GetOrigin())
	if err != nil {
		return nil, err
	}
	if in.GetStatus() == system.TenantDomainStatus_TENANT_DOMAIN_STATUS_UNKNOWN {
		in.Status = system.TenantDomainStatus_TENANT_DOMAIN_STATUS_ACTIVE
	}
	if in.GetStatus() == system.TenantDomainStatus_TENANT_DOMAIN_STATUS_ACTIVE {
		count, err := l.svcCtx.TenantDomainModel.CountActive(l.ctx, tenant, 0)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "count active domains failed: %v", err)
		}
		if count > 0 {
			return nil, status.Error(codes.FailedPrecondition, "tenant already has an active domain")
		}
	}
	now := time.Now().UnixMilli()
	if err := l.svcCtx.TenantDomainModel.Insert(l.ctx, &models.SysTenantDomain{TenantId: tenant, Origin: strings.TrimRight(origin, "/"), Status: int64(in.GetStatus()), Priority: in.GetPriority(), CreateTimes: now, UpdateTimes: now}); err != nil {
		return nil, status.Errorf(codes.Internal, "create tenant domain failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
