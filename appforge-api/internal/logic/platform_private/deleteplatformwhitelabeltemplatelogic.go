// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeletePlatformWhiteLabelTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeletePlatformWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeletePlatformWhiteLabelTemplateLogic {
	return &DeletePlatformWhiteLabelTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeletePlatformWhiteLabelTemplateLogic) DeletePlatformWhiteLabelTemplate(req *types.PlatformIdReq) (resp *types.RespBase, err error) {
	return deletePlatformWhiteLabelTemplate(l.ctx, l.svcCtx, req.Id)
}
