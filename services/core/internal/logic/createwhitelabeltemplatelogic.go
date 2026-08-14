package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateWhiteLabelTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWhiteLabelTemplateLogic {
	return &CreateWhiteLabelTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建V3白标模板。
func (l *CreateWhiteLabelTemplateLogic) CreateWhiteLabelTemplate(in *core.CreateWhiteLabelTemplateReq) (*core.WhiteLabelTemplateResp, error) {
	return createWhiteLabelTemplate(l.ctx, l.svcCtx, in)
}
