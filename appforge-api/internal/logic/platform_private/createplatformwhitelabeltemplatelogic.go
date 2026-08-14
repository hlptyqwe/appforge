// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformWhiteLabelTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformWhiteLabelTemplateLogic {
	return &CreatePlatformWhiteLabelTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformWhiteLabelTemplateLogic) CreatePlatformWhiteLabelTemplate(req *types.CreatePlatformWhiteLabelTemplateReq) (resp *types.PlatformWhiteLabelTemplateResp, err error) {
	return createPlatformWhiteLabelTemplate(l.ctx, l.svcCtx, req)
}
