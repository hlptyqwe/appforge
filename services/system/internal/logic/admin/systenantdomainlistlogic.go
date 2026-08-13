package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysTenantDomainListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantDomainListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantDomainListLogic {
	return &SysTenantDomainListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantDomainListLogic) SysTenantDomainList(in *system.SysTenantDomainListReq) (*system.SysTenantDomainListResp, error) {
	if in == nil || in.GetTenantId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	tenant, err := effectiveTenant(l.ctx, in.GetTenantId())
	if err != nil {
		return nil, err
	}
	rows, err := l.svcCtx.TenantDomainModel.FindAllByTenant(l.ctx, tenant)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tenant domains failed: %v", err)
	}
	data := make([]*system.SysTenantDomainItem, 0, len(rows))
	for _, row := range rows {
		data = append(data, domainItem(row))
	}
	return &system.SysTenantDomainListResp{Base: responseBase(), Data: data}, nil
}
