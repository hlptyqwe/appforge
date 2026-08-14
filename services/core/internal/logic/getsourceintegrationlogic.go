package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSourceIntegrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSourceIntegrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSourceIntegrationLogic {
	return &GetSourceIntegrationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询代码平台集成。
func (l *GetSourceIntegrationLogic) GetSourceIntegration(in *core.SourceIntegrationIdReq) (*core.SourceIntegrationResp, error) {
	return getSourceIntegration(l.ctx, l.svcCtx, in)
}
