package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSourceIntegrationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSourceIntegrationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSourceIntegrationsLogic {
	return &ListSourceIntegrationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询代码平台集成。
func (l *ListSourceIntegrationsLogic) ListSourceIntegrations(in *core.SourceIntegrationListReq) (*core.SourceIntegrationListResp, error) {
	return listSourceIntegrations(l.ctx, l.svcCtx, in)
}
