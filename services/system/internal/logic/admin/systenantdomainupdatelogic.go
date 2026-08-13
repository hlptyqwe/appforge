package adminlogic

import (
	"context"
	"time"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"
	"appforge/services/system/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysTenantDomainUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantDomainUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantDomainUpdateLogic {
	return &SysTenantDomainUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantDomainUpdateLogic) SysTenantDomainUpdate(in *system.SysTenantDomainUpdateReq) (*system.RespBase, error) {
	if in == nil || in.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	item, err := l.svcCtx.TenantDomainModel.FindOne(l.ctx, in.GetId())
	if err != nil {
		return nil, notFound(err, "tenant domain")
	}
	if _, err := effectiveTenant(l.ctx, item.TenantId); err != nil {
		return nil, err
	}
	if in.GetOrigin() != "" {
		item.Origin, err = normalizeOrigin(in.GetOrigin())
		if err != nil {
			return nil, err
		}
	}
	if in.GetStatus() != system.TenantDomainStatus_TENANT_DOMAIN_STATUS_UNKNOWN {
		item.Status = int64(in.GetStatus())
	}
	if item.Status == models.TenantDomainStatusActive {
		count, err := l.svcCtx.TenantDomainModel.CountActive(l.ctx, item.TenantId, item.Id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "count active domains failed: %v", err)
		}
		if count > 0 {
			return nil, status.Error(codes.FailedPrecondition, "tenant already has an active domain")
		}
	}
	if in.GetPriority() != 0 {
		item.Priority = in.GetPriority()
	}
	item.UpdateTimes = time.Now().UnixMilli()
	if err := l.svcCtx.TenantDomainModel.Update(l.ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update tenant domain failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
