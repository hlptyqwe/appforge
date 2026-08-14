package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CopyWhiteLabelTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCopyWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CopyWhiteLabelTemplateLogic {
	return &CopyWhiteLabelTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 复制V3白标模板及其修订为新草稿。
func (l *CopyWhiteLabelTemplateLogic) CopyWhiteLabelTemplate(in *core.CopyWhiteLabelTemplateReq) (*core.WhiteLabelTemplateResp, error) {
	return copyWhiteLabelTemplate(l.ctx, l.svcCtx, in)
}
