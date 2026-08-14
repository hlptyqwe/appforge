// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformWhiteLabelTemplateRevisionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformWhiteLabelTemplateRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformWhiteLabelTemplateRevisionLogic {
	return &CreatePlatformWhiteLabelTemplateRevisionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformWhiteLabelTemplateRevisionLogic) CreatePlatformWhiteLabelTemplateRevision(req *types.CreatePlatformWhiteLabelTemplateRevisionReq) (resp *types.PlatformWhiteLabelTemplateRevisionResp, err error) {
	return createPlatformWhiteLabelTemplateRevision(l.ctx, l.svcCtx, req)
}
