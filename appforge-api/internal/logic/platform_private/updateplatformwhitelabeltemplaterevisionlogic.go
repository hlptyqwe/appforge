// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdatePlatformWhiteLabelTemplateRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdatePlatformWhiteLabelTemplateRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdatePlatformWhiteLabelTemplateRevisionLogic {
	return &UpdatePlatformWhiteLabelTemplateRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdatePlatformWhiteLabelTemplateRevisionLogic) UpdatePlatformWhiteLabelTemplateRevision(req *types.UpdatePlatformWhiteLabelTemplateRevisionReq) (resp *types.PlatformWhiteLabelTemplateRevisionResp, err error) {
	return updatePlatformWhiteLabelTemplateRevision(l.ctx, l.svcCtx, req)
}
