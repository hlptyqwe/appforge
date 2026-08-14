package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteWhiteLabelProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteWhiteLabelProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteWhiteLabelProductLogic {
	return &DeleteWhiteLabelProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 删除未启用且没有历史构建的V3白标产品。
func (l *DeleteWhiteLabelProductLogic) DeleteWhiteLabelProduct(in *core.WhiteLabelProductIdReq) (*core.RespBase, error) {
	return deleteWhiteLabelProduct(l.ctx, l.svcCtx, in)
}
