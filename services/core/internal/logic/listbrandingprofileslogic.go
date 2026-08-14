package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBrandingProfilesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBrandingProfilesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBrandingProfilesLogic {
	return &ListBrandingProfilesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询当前租户的品牌配置。
func (l *ListBrandingProfilesLogic) ListBrandingProfiles(in *core.BrandingProfileListReq) (*core.BrandingProfileListResp, error) {
	return listBrandingProfiles(l.ctx, l.svcCtx, in)
}
