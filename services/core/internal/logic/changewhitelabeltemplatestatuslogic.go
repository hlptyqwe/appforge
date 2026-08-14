package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChangeWhiteLabelTemplateStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewChangeWhiteLabelTemplateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChangeWhiteLabelTemplateStatusLogic {
	return &ChangeWhiteLabelTemplateStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 修改V3白标模板状态。
func (l *ChangeWhiteLabelTemplateStatusLogic) ChangeWhiteLabelTemplateStatus(in *core.ChangeWhiteLabelTemplateStatusReq) (*core.WhiteLabelTemplateResp, error) {
	return changeWhiteLabelTemplateStatus(l.ctx, l.svcCtx, in)
}
