// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1GetWhiteLabelProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1GetWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1GetWhiteLabelProductLogic {
	return &OpenV1GetWhiteLabelProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1GetWhiteLabelProductLogic) OpenV1GetWhiteLabelProduct(req *types.PlatformIdReq) (resp *types.PlatformWhiteLabelProductResp, err error) {
	return openV1GetWhiteLabelProduct(l.ctx, l.svcCtx, req)
}
