package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateWhiteLabelTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateWhiteLabelTemplateLogic {
	return &UpdateWhiteLabelTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新尚未发布的V3白标模板。
func (l *UpdateWhiteLabelTemplateLogic) UpdateWhiteLabelTemplate(in *core.UpdateWhiteLabelTemplateReq) (*core.WhiteLabelTemplateResp, error) {
	return updateWhiteLabelTemplate(l.ctx, l.svcCtx, in)
}
