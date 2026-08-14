// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformWhiteLabelTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformWhiteLabelTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformWhiteLabelTemplatesLogic {
	return &ListPlatformWhiteLabelTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformWhiteLabelTemplatesLogic) ListPlatformWhiteLabelTemplates(req *types.ListPlatformWhiteLabelTemplatesReq) (resp *types.PlatformWhiteLabelTemplateListResp, err error) {
	return listPlatformWhiteLabelTemplates(l.ctx, l.svcCtx, req)
}
