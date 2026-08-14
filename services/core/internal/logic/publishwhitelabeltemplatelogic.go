package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type PublishWhiteLabelTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPublishWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PublishWhiteLabelTemplateLogic {
	return &PublishWhiteLabelTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 发布模板修订。
func (l *PublishWhiteLabelTemplateLogic) PublishWhiteLabelTemplate(in *core.PublishWhiteLabelTemplateReq) (*core.WhiteLabelTemplateResp, error) {
	return publishWhiteLabelTemplate(l.ctx, l.svcCtx, in)
}
