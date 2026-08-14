// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1UpdateWhiteLabelProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1UpdateWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1UpdateWhiteLabelProductLogic {
	return &OpenV1UpdateWhiteLabelProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1UpdateWhiteLabelProductLogic) OpenV1UpdateWhiteLabelProduct(req *types.UpdatePlatformWhiteLabelProductReq) (resp *types.PlatformWhiteLabelProductResp, err error) {
	return openV1UpdateWhiteLabelProduct(l.ctx, l.svcCtx, req)
}
