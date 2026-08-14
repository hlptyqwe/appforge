// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformWhiteLabelProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformWhiteLabelProductLogic {
	return &CreatePlatformWhiteLabelProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformWhiteLabelProductLogic) CreatePlatformWhiteLabelProduct(req *types.CreatePlatformWhiteLabelProductReq) (resp *types.PlatformWhiteLabelProductResp, err error) {
	return createPlatformWhiteLabelProduct(l.ctx, l.svcCtx, req)
}
