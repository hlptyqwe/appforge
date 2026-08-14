package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteWhiteLabelTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteWhiteLabelTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteWhiteLabelTemplateLogic {
	return &DeleteWhiteLabelTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 删除从未发布且未被产品引用的V3白标模板。
func (l *DeleteWhiteLabelTemplateLogic) DeleteWhiteLabelTemplate(in *core.WhiteLabelTemplateIdReq) (*core.RespBase, error) {
	return deleteWhiteLabelTemplate(l.ctx, l.svcCtx, in)
}
