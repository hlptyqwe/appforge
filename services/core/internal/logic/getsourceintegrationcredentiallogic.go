package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSourceIntegrationCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSourceIntegrationCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSourceIntegrationCredentialLogic {
	return &GetSourceIntegrationCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 读取有效集成令牌，仅供受内部认证保护的供应商客户端调用，禁止暴露到HTTP。
func (l *GetSourceIntegrationCredentialLogic) GetSourceIntegrationCredential(in *core.SourceIntegrationIdReq) (*core.SourceIntegrationCredentialResp, error) {
	return getSourceIntegrationCredential(l.ctx, l.svcCtx, in)
}
