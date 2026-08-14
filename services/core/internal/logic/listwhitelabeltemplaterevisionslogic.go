package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWhiteLabelTemplateRevisionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWhiteLabelTemplateRevisionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWhiteLabelTemplateRevisionsLogic {
	return &ListWhiteLabelTemplateRevisionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询模板修订。
func (l *ListWhiteLabelTemplateRevisionsLogic) ListWhiteLabelTemplateRevisions(in *core.WhiteLabelTemplateRevisionListReq) (*core.WhiteLabelTemplateRevisionListResp, error) {
	return listWhiteLabelTemplateRevisions(l.ctx, l.svcCtx, in)
}
