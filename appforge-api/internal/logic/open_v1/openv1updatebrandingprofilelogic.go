// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1UpdateBrandingProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1UpdateBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1UpdateBrandingProfileLogic {
	return &OpenV1UpdateBrandingProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1UpdateBrandingProfileLogic) OpenV1UpdateBrandingProfile(req *types.UpdatePlatformBrandingProfileReq) (resp *types.PlatformBrandingProfileResp, err error) {
	return openV1UpdateBrandingProfile(l.ctx, l.svcCtx, req)
}
