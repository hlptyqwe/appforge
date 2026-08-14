package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBrandingPreflightLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBrandingPreflightLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBrandingPreflightLogic {
	return &GetBrandingPreflightLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询品牌兼容性预检记录。
func (l *GetBrandingPreflightLogic) GetBrandingPreflight(in *core.BrandingPreflightIdReq) (*core.BrandingPreflightResp, error) {
	return getBrandingPreflight(l.ctx, l.svcCtx, in)
}
