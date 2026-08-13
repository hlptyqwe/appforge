package adminlogic

import (
	"context"

	"appforge/proto/system"
	"appforge/services/system/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SysTenantListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSysTenantListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SysTenantListLogic {
	return &SysTenantListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SysTenantListLogic) SysTenantList(in *system.SysTenantListReq) (*system.SysTenantListResp, error) {
	// todo: add your logic here and delete this line

	return &system.SysTenantListResp{}, nil
}
