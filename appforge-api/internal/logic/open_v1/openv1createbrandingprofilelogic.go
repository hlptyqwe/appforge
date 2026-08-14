// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package open_v1

import (
	"context"

	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpenV1CreateBrandingProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpenV1CreateBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpenV1CreateBrandingProfileLogic {
	return &OpenV1CreateBrandingProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpenV1CreateBrandingProfileLogic) OpenV1CreateBrandingProfile(req *types.CreatePlatformBrandingProfileReq) (resp *types.PlatformBrandingProfileResp, err error) {
	return openV1CreateBrandingProfile(l.ctx, l.svcCtx, req)
}
