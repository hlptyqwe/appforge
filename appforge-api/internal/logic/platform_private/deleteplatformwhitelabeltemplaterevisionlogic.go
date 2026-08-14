// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeletePlatformWhiteLabelTemplateRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeletePlatformWhiteLabelTemplateRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeletePlatformWhiteLabelTemplateRevisionLogic {
	return &DeletePlatformWhiteLabelTemplateRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeletePlatformWhiteLabelTemplateRevisionLogic) DeletePlatformWhiteLabelTemplateRevision(req *types.PlatformWhiteLabelTemplateRevisionIdReq) (resp *types.RespBase, err error) {
	return deletePlatformWhiteLabelTemplateRevision(l.ctx, l.svcCtx, req)
}
