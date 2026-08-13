package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SysTenantDomainDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantDomainDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantDomainDeleteLogic {
	return &SysTenantDomainDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantDomainDeleteLogic) SysTenantDomainDelete(in *system.SysTenantDomainDeleteReq) (*system.RespBase, error) {
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
	if err := l.svcCtx.TenantDomainModel.Delete(l.ctx, item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete tenant domain failed: %v", err)
	}
	return &system.RespBase{Base: responseBase()}, nil
}
