package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteBrandingPreflightLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteBrandingPreflightLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteBrandingPreflightLogic {
	return &CompleteBrandingPreflightLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 完成品牌兼容性预检。
func (l *CompleteBrandingPreflightLogic) CompleteBrandingPreflight(in *core.CompleteBrandingPreflightReq) (*core.BrandingPreflightResp, error) {
	return completeBrandingPreflight(l.ctx, l.svcCtx, in)
}
