package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateBrandingPreflightLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateBrandingPreflightLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateBrandingPreflightLogic {
	return &CreateBrandingPreflightLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建或重置品牌兼容性预检记录。
func (l *CreateBrandingPreflightLogic) CreateBrandingPreflight(in *core.CreateBrandingPreflightReq) (*core.BrandingPreflightResp, error) {
	return createBrandingPreflight(l.ctx, l.svcCtx, in)
}
