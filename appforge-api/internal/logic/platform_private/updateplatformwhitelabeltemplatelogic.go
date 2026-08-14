// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePlatformWhiteLabelTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePlatformWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePlatformWhiteLabelTemplateLogic {
	return &UpdatePlatformWhiteLabelTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePlatformWhiteLabelTemplateLogic) UpdatePlatformWhiteLabelTemplate(req *types.UpdatePlatformWhiteLabelTemplateReq) (resp *types.PlatformWhiteLabelTemplateResp, err error) {
	return updatePlatformWhiteLabelTemplate(l.ctx, l.svcCtx, req)
}
