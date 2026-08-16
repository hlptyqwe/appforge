// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ImportPlatformAirGappedResultLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewImportPlatformAirGappedResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportPlatformAirGappedResultLogic {
	return &ImportPlatformAirGappedResultLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ImportPlatformAirGappedResultLogic) ImportPlatformAirGappedResult(req *types.ImportPlatformAirGappedResultReq) (resp *types.PlatformAirGappedPackageResp, err error) {
	return importAirGappedResult(l.ctx, l.svcCtx, req)
}
