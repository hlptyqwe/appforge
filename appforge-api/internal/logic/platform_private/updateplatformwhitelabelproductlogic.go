// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePlatformWhiteLabelProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePlatformWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePlatformWhiteLabelProductLogic {
	return &UpdatePlatformWhiteLabelProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePlatformWhiteLabelProductLogic) UpdatePlatformWhiteLabelProduct(req *types.UpdatePlatformWhiteLabelProductReq) (resp *types.PlatformWhiteLabelProductResp, err error) {
	return updatePlatformWhiteLabelProduct(l.ctx, l.svcCtx, req)
}
