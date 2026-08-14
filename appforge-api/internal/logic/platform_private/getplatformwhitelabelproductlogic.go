// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformWhiteLabelProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformWhiteLabelProductLogic {
	return &GetPlatformWhiteLabelProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformWhiteLabelProductLogic) GetPlatformWhiteLabelProduct(req *types.PlatformIdReq) (resp *types.PlatformWhiteLabelProductResp, err error) {
	return getPlatformWhiteLabelProduct(l.ctx, l.svcCtx, req.Id)
}
