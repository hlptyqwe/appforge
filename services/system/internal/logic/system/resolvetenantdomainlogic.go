package systemlogic

import (
	"context"
	"strings"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResolveTenantDomainLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveTenantDomainLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveTenantDomainLogic {
	return &ResolveTenantDomainLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ResolveTenantDomainLogic) ResolveTenantDomain(in *system.ResolveTenantDomainReq) (*system.ResolveTenantDomainResp, error) {
	if in == nil || in.GetTenantId() <= 0 || strings.TrimSpace(in.GetSourceOrigin()) == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id and source_origin are required")
	}
	source, err := l.svcCtx.TenantDomainModel.FindByTenantOrigin(l.ctx, in.GetTenantId(), strings.TrimRight(strings.TrimSpace(in.GetSourceOrigin()), "/"))
	if err != nil {
		return nil, status.Error(codes.NotFound, "tenant domain not found")
	}
	result := &system.ResolveTenantDomainResp{Base: responseBase(), SourceStatus: system.TenantDomainStatus(source.Status), TargetOrigin: source.Origin}
	if source.Status != models.TenantDomainStatusActive {
		target, err := l.svcCtx.TenantDomainModel.FindHighestPriorityActive(l.ctx, in.GetTenantId())
		if err != nil {
			return nil, status.Error(codes.FailedPrecondition, "tenant has no active domain")
		}
		result.TargetOrigin = target.Origin
	}
	return result, nil
}
