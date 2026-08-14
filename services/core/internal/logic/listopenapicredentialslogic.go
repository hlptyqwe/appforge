package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListOpenApiCredentialsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListOpenApiCredentialsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOpenApiCredentialsLogic {
	return &ListOpenApiCredentialsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询V5 Open API凭证。
func (l *ListOpenApiCredentialsLogic) ListOpenApiCredentials(in *core.OpenApiCredentialListReq) (*core.OpenApiCredentialListResp, error) {
	return listOpenApiCredentials(l.ctx, l.svcCtx, in)
}
