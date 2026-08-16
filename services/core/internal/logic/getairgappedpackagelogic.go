package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAirGappedPackageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAirGappedPackageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAirGappedPackageLogic {
	return &GetAirGappedPackageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询当前租户AIR_GAPPED离线包状态。
func (l *GetAirGappedPackageLogic) GetAirGappedPackage(in *core.AirGappedPackageReq) (*core.AirGappedPackageResp, error) {
	return getAirGappedPackage(l.ctx, l.svcCtx, in)
}
