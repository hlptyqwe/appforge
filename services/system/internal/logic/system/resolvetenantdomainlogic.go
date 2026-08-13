package systemlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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
	// todo: add your logic here and delete this line

	return &system.ResolveTenantDomainResp{}, nil
}
