package applogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return &system.SysTenantDetailResp{}, nil
}
