package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RevokeLocalAgentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevokeLocalAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeLocalAgentLogic {
	return &RevokeLocalAgentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 管理员吊销Agent及其全部有效证书。
func (l *RevokeLocalAgentLogic) RevokeLocalAgent(in *core.RevokeLocalAgentReq) (*core.LocalAgentResp, error) {
	return revokeLocalAgent(l.ctx, l.svcCtx, in)
}
