package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateOpenApiCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOpenApiCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOpenApiCredentialLogic {
	return &CreateOpenApiCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建V5 Open API凭证并一次性返回Secret。
func (l *CreateOpenApiCredentialLogic) CreateOpenApiCredential(in *core.CreateOpenApiCredentialReq) (*core.OpenApiCredentialSecretResp, error) {
	return createOpenApiCredential(l.ctx, l.svcCtx, in)
}
