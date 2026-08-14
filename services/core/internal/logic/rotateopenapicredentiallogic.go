package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RotateOpenApiCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRotateOpenApiCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotateOpenApiCredentialLogic {
	return &RotateOpenApiCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 轮换V5 Open API凭证并一次性返回新Secret。
func (l *RotateOpenApiCredentialLogic) RotateOpenApiCredential(in *core.RotateOpenApiCredentialReq) (*core.OpenApiCredentialSecretResp, error) {
	return rotateOpenApiCredential(l.ctx, l.svcCtx, in)
}
