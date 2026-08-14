// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangePlatformWhiteLabelProductStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangePlatformWhiteLabelProductStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePlatformWhiteLabelProductStatusLogic {
	return &ChangePlatformWhiteLabelProductStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChangePlatformWhiteLabelProductStatusLogic) ChangePlatformWhiteLabelProductStatus(req *types.ChangePlatformWhiteLabelProductStatusReq) (resp *types.PlatformWhiteLabelProductResp, err error) {
	return changePlatformWhiteLabelProductStatus(l.ctx, l.svcCtx, req)
}
