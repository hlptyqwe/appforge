// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1GetBrandingProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1GetBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1GetBrandingProfileLogic {
	return &OpenV1GetBrandingProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1GetBrandingProfileLogic) OpenV1GetBrandingProfile(req *types.PlatformIdReq) (resp *types.PlatformBrandingProfileResp, err error) {
	return openV1GetBrandingProfile(l.ctx, l.svcCtx, req)
}
