package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBrandingPreflightsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBrandingPreflightsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBrandingPreflightsLogic {
	return &ListBrandingPreflightsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询品牌兼容性预检记录。
func (l *ListBrandingPreflightsLogic) ListBrandingPreflights(in *core.BrandingPreflightListReq) (*core.BrandingPreflightListResp, error) {
	return listBrandingPreflights(l.ctx, l.svcCtx, in)
}
