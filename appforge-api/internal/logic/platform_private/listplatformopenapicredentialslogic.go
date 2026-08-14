// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPlatformOpenApiCredentialsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformOpenApiCredentialsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformOpenApiCredentialsLogic {
	return &ListPlatformOpenApiCredentialsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformOpenApiCredentialsLogic) ListPlatformOpenApiCredentials(req *types.ListPlatformOpenApiCredentialsReq) (resp *types.PlatformOpenApiCredentialListResp, err error) {
	result, err := l.svcCtx.CoreCli.ListOpenApiCredentials(l.ctx, &core.OpenApiCredentialListReq{
		Page: platformlogic.PlatformPage(req.PageReq), Status: core.OpenApiCredentialStatus(req.Status), Keyword: req.Keyword,
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformOpenApiCredential, 0, len(result.Data))
	for _, item := range result.Data {
		data = append(data, platformlogic.MapPlatformOpenApiCredential(item))
	}
	return &types.PlatformOpenApiCredentialListResp{RespBase: platformlogic.PlatformRespBase(result.Base), Data: data}, nil
}
