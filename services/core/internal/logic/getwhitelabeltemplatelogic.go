package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWhiteLabelTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWhiteLabelTemplateLogic {
	return &GetWhiteLabelTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询V3白标模板。
func (l *GetWhiteLabelTemplateLogic) GetWhiteLabelTemplate(in *core.WhiteLabelTemplateIdReq) (*core.WhiteLabelTemplateResp, error) {
	return getWhiteLabelTemplate(l.ctx, l.svcCtx, in)
}
