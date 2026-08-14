// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformWhiteLabelTemplateRevisionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformWhiteLabelTemplateRevisionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformWhiteLabelTemplateRevisionsLogic {
	return &ListPlatformWhiteLabelTemplateRevisionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformWhiteLabelTemplateRevisionsLogic) ListPlatformWhiteLabelTemplateRevisions(req *types.ListPlatformWhiteLabelTemplateRevisionsReq) (resp *types.PlatformWhiteLabelTemplateRevisionListResp, err error) {
	return listPlatformWhiteLabelTemplateRevisions(l.ctx, l.svcCtx, req)
}
