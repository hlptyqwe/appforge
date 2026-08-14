// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangePlatformBrandingProfileStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChangePlatformBrandingProfileStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangePlatformBrandingProfileStatusLogic {
	return &ChangePlatformBrandingProfileStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChangePlatformBrandingProfileStatusLogic) ChangePlatformBrandingProfileStatus(req *types.ChangePlatformBrandingProfileStatusReq) (resp *types.PlatformBrandingProfileResp, err error) {
	return changePlatformBrandingProfileStatus(l.ctx, l.svcCtx, req)
}
