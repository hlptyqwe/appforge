package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBrandingProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBrandingProfileLogic {
	return &GetBrandingProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询当前租户的品牌配置。
func (l *GetBrandingProfileLogic) GetBrandingProfile(in *core.BrandingProfileIdReq) (*core.BrandingProfileResp, error) {
	return getBrandingProfile(l.ctx, l.svcCtx, in)
}
