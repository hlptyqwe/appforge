// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PreflightPlatformWhiteLabelProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPreflightPlatformWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreflightPlatformWhiteLabelProductLogic {
	return &PreflightPlatformWhiteLabelProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PreflightPlatformWhiteLabelProductLogic) PreflightPlatformWhiteLabelProduct(req *types.PlatformIdReq) (resp *types.PlatformWhiteLabelProductPreflightResp, err error) {
	return preflightPlatformWhiteLabelProduct(l.ctx, l.svcCtx, req.Id)
}
