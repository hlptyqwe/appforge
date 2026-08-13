package applogic

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

type SysTenantDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantDetailLogic {
	return &SysTenantDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantDetailLogic) SysTenantDetail(in *system.SysTenantDetailReq) (*system.SysTenantDetailResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	var (
		item *models.SysTenant
		err  error
	)
	if in.TenantId != nil {
		item, err = l.svcCtx.TenantMode.FindOne(l.ctx, in.GetTenantId())
	} else if in.TenantCode != nil && strings.TrimSpace(in.GetTenantCode()) != "" {
		item, err = l.svcCtx.TenantMode.FindByTenantCode(l.ctx, strings.TrimSpace(in.GetTenantCode()))
	} else {
		return nil, status.Error(codes.InvalidArgument, "tenant_id or tenant_code is required")
	}
	if err != nil {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}
	return &system.SysTenantDetailResp{Base: responseBase(), Data: tenantItem(item)}, nil
}
