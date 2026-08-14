package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthenticateOpenApiCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthenticateOpenApiCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthenticateOpenApiCredentialLogic {
	return &AuthenticateOpenApiCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 验证V5 Open API凭证、IP、状态和有效期。
func (l *AuthenticateOpenApiCredentialLogic) AuthenticateOpenApiCredential(in *core.AuthenticateOpenApiCredentialReq) (*core.OpenApiAuthContextResp, error) {
	return authenticateOpenApiCredential(l.ctx, l.svcCtx, in)
}
