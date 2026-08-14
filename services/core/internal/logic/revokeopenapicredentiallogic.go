package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RevokeOpenApiCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevokeOpenApiCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeOpenApiCredentialLogic {
	return &RevokeOpenApiCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 立即吊销V5 Open API凭证。
func (l *RevokeOpenApiCredentialLogic) RevokeOpenApiCredential(in *core.OpenApiCredentialIdReq) (*core.OpenApiCredentialResp, error) {
	return revokeOpenApiCredential(l.ctx, l.svcCtx, in)
}
