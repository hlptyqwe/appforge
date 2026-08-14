// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformWhiteLabelTemplateRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformWhiteLabelTemplateRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformWhiteLabelTemplateRevisionLogic {
	return &GetPlatformWhiteLabelTemplateRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformWhiteLabelTemplateRevisionLogic) GetPlatformWhiteLabelTemplateRevision(req *types.PlatformWhiteLabelTemplateRevisionIdReq) (resp *types.PlatformWhiteLabelTemplateRevisionResp, err error) {
	return getPlatformWhiteLabelTemplateRevision(l.ctx, l.svcCtx, req)
}
