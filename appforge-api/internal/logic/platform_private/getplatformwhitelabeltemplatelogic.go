// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformWhiteLabelTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformWhiteLabelTemplateLogic {
	return &GetPlatformWhiteLabelTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformWhiteLabelTemplateLogic) GetPlatformWhiteLabelTemplate(req *types.PlatformIdReq) (resp *types.PlatformWhiteLabelTemplateResp, err error) {
	return getPlatformWhiteLabelTemplate(l.ctx, l.svcCtx, req.Id)
}
