// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CopyPlatformWhiteLabelTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCopyPlatformWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CopyPlatformWhiteLabelTemplateLogic {
	return &CopyPlatformWhiteLabelTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CopyPlatformWhiteLabelTemplateLogic) CopyPlatformWhiteLabelTemplate(req *types.CopyPlatformWhiteLabelTemplateReq) (resp *types.PlatformWhiteLabelTemplateResp, err error) {
	return copyPlatformWhiteLabelTemplate(l.ctx, l.svcCtx, req)
}
