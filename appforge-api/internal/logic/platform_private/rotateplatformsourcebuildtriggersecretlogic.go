// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RotatePlatformSourceBuildTriggerSecretLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRotatePlatformSourceBuildTriggerSecretLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotatePlatformSourceBuildTriggerSecretLogic {
	return &RotatePlatformSourceBuildTriggerSecretLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RotatePlatformSourceBuildTriggerSecretLogic) RotatePlatformSourceBuildTriggerSecret(req *types.PlatformIdReq) (resp *types.PlatformSourceBuildTriggerSecretResp, err error) {
	return rotatePlatformSourceBuildTriggerSecret(l.ctx, l.svcCtx, req.Id)
}
