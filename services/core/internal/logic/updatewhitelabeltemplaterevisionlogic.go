package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateWhiteLabelTemplateRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateWhiteLabelTemplateRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateWhiteLabelTemplateRevisionLogic {
	return &UpdateWhiteLabelTemplateRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新尚未发布的模板草稿修订。
func (l *UpdateWhiteLabelTemplateRevisionLogic) UpdateWhiteLabelTemplateRevision(in *core.UpdateWhiteLabelTemplateRevisionReq) (*core.WhiteLabelTemplateRevisionResp, error) {
	return updateWhiteLabelTemplateRevision(l.ctx, l.svcCtx, in)
}
