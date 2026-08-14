// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1CreateWhiteLabelProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1CreateWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1CreateWhiteLabelProductLogic {
	return &OpenV1CreateWhiteLabelProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1CreateWhiteLabelProductLogic) OpenV1CreateWhiteLabelProduct(req *types.CreatePlatformWhiteLabelProductReq) (resp *types.PlatformWhiteLabelProductResp, err error) {
	return openV1CreateWhiteLabelProduct(l.ctx, l.svcCtx, req)
}
