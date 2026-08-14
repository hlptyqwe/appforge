package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClaimBrandingPreflightLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClaimBrandingPreflightLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClaimBrandingPreflightLogic {
	return &ClaimBrandingPreflightLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 原子领取一个待处理或租约过期的品牌兼容性预检。
func (l *ClaimBrandingPreflightLogic) ClaimBrandingPreflight(in *core.ClaimBrandingPreflightReq) (*core.BrandingPreflightExecutionContextResp, error) {
	return claimBrandingPreflight(l.ctx, l.svcCtx, in)
}
