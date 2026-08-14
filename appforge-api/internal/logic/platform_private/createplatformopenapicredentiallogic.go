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

type CreatePlatformOpenApiCredentialLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformOpenApiCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformOpenApiCredentialLogic {
	return &CreatePlatformOpenApiCredentialLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformOpenApiCredentialLogic) CreatePlatformOpenApiCredential(req *types.CreatePlatformOpenApiCredentialReq) (resp *types.PlatformOpenApiCredentialSecretResp, err error) {
	scopes := make([]core.OpenApiScope, 0, len(req.Scopes))
	for _, scope := range req.Scopes {
		scopes = append(scopes, core.OpenApiScope(scope))
	}
	result, err := l.svcCtx.CoreCli.CreateOpenApiCredential(l.ctx, &core.CreateOpenApiCredentialReq{
		CredentialName: req.CredentialName, Scopes: scopes, AppIds: req.AppIds,
		IpAllowlist: req.IpAllowlist, RateLimitPerMinute: req.RateLimitPerMinute, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	data := result.Data
	if data == nil {
		data = &core.OpenApiCredentialSecret{}
	}
	return &types.PlatformOpenApiCredentialSecretResp{RespBase: platformlogic.PlatformRespBase(result.Base),
		Data: types.PlatformOpenApiCredentialSecret{Credential: platformlogic.MapPlatformOpenApiCredential(data.Credential), ApiKey: data.ApiKey}}, nil
}
