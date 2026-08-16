// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformAirGappedPackageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformAirGappedPackageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformAirGappedPackageLogic {
	return &GetPlatformAirGappedPackageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformAirGappedPackageLogic) GetPlatformAirGappedPackage(req *types.PlatformAirGappedPackageReq) (resp *types.PlatformAirGappedPackageResp, err error) {
	item, err := l.svcCtx.CoreCli.GetAirGappedPackage(l.ctx, &core.AirGappedPackageReq{PackageCode: req.PackageCode})
	if err != nil {
		return nil, err
	}
	return &types.PlatformAirGappedPackageResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: mapPlatformAirGappedPackage(item.Data)}, nil
}
