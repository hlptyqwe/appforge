package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetWhiteLabelTemplateRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWhiteLabelTemplateRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWhiteLabelTemplateRevisionLogic {
	return &GetWhiteLabelTemplateRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询单个模板修订。
func (l *GetWhiteLabelTemplateRevisionLogic) GetWhiteLabelTemplateRevision(in *core.WhiteLabelTemplateRevisionIdReq) (*core.WhiteLabelTemplateRevisionResp, error) {
	return getWhiteLabelTemplateRevision(l.ctx, l.svcCtx, in)
}
