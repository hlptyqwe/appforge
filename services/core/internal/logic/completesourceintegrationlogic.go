package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteSourceIntegrationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteSourceIntegrationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteSourceIntegrationLogic {
	return &CompleteSourceIntegrationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 完成受信任的GitHub/GitLab OAuth回调或App安装同步，并加密保存令牌。
func (l *CompleteSourceIntegrationLogic) CompleteSourceIntegration(in *core.CompleteSourceIntegrationReq) (*core.SourceIntegrationResp, error) {
	return completeSourceIntegration(l.ctx, l.svcCtx, in)
}
