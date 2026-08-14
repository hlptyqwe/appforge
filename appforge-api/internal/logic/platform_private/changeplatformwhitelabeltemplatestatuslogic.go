// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangePlatformWhiteLabelTemplateStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangePlatformWhiteLabelTemplateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePlatformWhiteLabelTemplateStatusLogic {
	return &ChangePlatformWhiteLabelTemplateStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChangePlatformWhiteLabelTemplateStatusLogic) ChangePlatformWhiteLabelTemplateStatus(req *types.ChangePlatformWhiteLabelTemplateStatusReq) (resp *types.PlatformWhiteLabelTemplateResp, err error) {
	return changePlatformWhiteLabelTemplateStatus(l.ctx, l.svcCtx, req)
}
