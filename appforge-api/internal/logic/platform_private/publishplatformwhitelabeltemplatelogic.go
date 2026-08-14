// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishPlatformWhiteLabelTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPublishPlatformWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishPlatformWhiteLabelTemplateLogic {
	return &PublishPlatformWhiteLabelTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PublishPlatformWhiteLabelTemplateLogic) PublishPlatformWhiteLabelTemplate(req *types.PublishPlatformWhiteLabelTemplateReq) (resp *types.PlatformWhiteLabelTemplateResp, err error) {
	return publishPlatformWhiteLabelTemplate(l.ctx, l.svcCtx, req)
}
