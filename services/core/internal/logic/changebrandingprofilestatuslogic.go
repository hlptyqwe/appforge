package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeBrandingProfileStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeBrandingProfileStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeBrandingProfileStatusLogic {
	return &ChangeBrandingProfileStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改品牌配置状态。
func (l *ChangeBrandingProfileStatusLogic) ChangeBrandingProfileStatus(in *core.ChangeBrandingProfileStatusReq) (*core.BrandingProfileResp, error) {
	return changeBrandingProfileStatus(l.ctx, l.svcCtx, in)
}
