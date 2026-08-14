package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWhiteLabelTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWhiteLabelTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWhiteLabelTemplatesLogic {
	return &ListWhiteLabelTemplatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询V3白标模板。
func (l *ListWhiteLabelTemplatesLogic) ListWhiteLabelTemplates(in *core.WhiteLabelTemplateListReq) (*core.WhiteLabelTemplateListResp, error) {
	return listWhiteLabelTemplates(l.ctx, l.svcCtx, in)
}
