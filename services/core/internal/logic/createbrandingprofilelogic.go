package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateBrandingProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateBrandingProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateBrandingProfileLogic {
	return &CreateBrandingProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建应用品牌配置。
func (l *CreateBrandingProfileLogic) CreateBrandingProfile(in *core.CreateBrandingProfileReq) (*core.BrandingProfileResp, error) {
	return createBrandingProfile(l.ctx, l.svcCtx, in)
}
