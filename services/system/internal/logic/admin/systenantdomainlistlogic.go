package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return &system.SysTenantDomainListResp{}, nil
}
