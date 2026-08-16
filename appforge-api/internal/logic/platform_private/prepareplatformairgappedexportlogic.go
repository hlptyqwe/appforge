// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PreparePlatformAirGappedExportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPreparePlatformAirGappedExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreparePlatformAirGappedExportLogic {
	return &PreparePlatformAirGappedExportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PreparePlatformAirGappedExportLogic) PreparePlatformAirGappedExport(req *types.PreparePlatformAirGappedExportReq) (resp *types.PlatformAirGappedExportResp, err error) {
	return prepareAirGappedExport(l.ctx, l.svcCtx, req)
}
