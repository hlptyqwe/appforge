package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateWhiteLabelTemplateRevisionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateWhiteLabelTemplateRevisionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWhiteLabelTemplateRevisionLogic {
	return &CreateWhiteLabelTemplateRevisionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建不可变模板修订。
func (l *CreateWhiteLabelTemplateRevisionLogic) CreateWhiteLabelTemplateRevision(in *core.CreateWhiteLabelTemplateRevisionReq) (*core.WhiteLabelTemplateRevisionResp, error) {
	return createWhiteLabelTemplateRevision(l.ctx, l.svcCtx, in)
}
