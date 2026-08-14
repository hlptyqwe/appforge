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

type RotatePlatformOpenApiCredentialLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRotatePlatformOpenApiCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotatePlatformOpenApiCredentialLogic {
	return &RotatePlatformOpenApiCredentialLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RotatePlatformOpenApiCredentialLogic) RotatePlatformOpenApiCredential(req *types.RotatePlatformOpenApiCredentialReq) (resp *types.PlatformOpenApiCredentialSecretResp, err error) {
	result, err := l.svcCtx.CoreCli.RotateOpenApiCredential(l.ctx, &core.RotateOpenApiCredentialReq{Id: req.Id, GraceSeconds: req.GraceSeconds})
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
