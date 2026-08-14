package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateBrandingProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateBrandingProfileLogic {
	return &UpdateBrandingProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新应用品牌配置并递增修订号。
func (l *UpdateBrandingProfileLogic) UpdateBrandingProfile(in *core.UpdateBrandingProfileReq) (*core.BrandingProfileResp, error) {
	return updateBrandingProfile(l.ctx, l.svcCtx, in)
}
