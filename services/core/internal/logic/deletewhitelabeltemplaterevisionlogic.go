package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteWhiteLabelTemplateRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteWhiteLabelTemplateRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteWhiteLabelTemplateRevisionLogic {
	return &DeleteWhiteLabelTemplateRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 删除尚未发布的模板草稿修订。
func (l *DeleteWhiteLabelTemplateRevisionLogic) DeleteWhiteLabelTemplateRevision(in *core.WhiteLabelTemplateRevisionIdReq) (*core.RespBase, error) {
	return deleteWhiteLabelTemplateRevision(l.ctx, l.svcCtx, in)
}
