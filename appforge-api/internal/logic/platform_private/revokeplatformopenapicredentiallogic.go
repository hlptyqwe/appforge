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

type RevokePlatformOpenApiCredentialLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRevokePlatformOpenApiCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokePlatformOpenApiCredentialLogic {
	return &RevokePlatformOpenApiCredentialLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RevokePlatformOpenApiCredentialLogic) RevokePlatformOpenApiCredential(req *types.RevokePlatformOpenApiCredentialReq) (resp *types.PlatformOpenApiCredentialResp, err error) {
	result, err := l.svcCtx.CoreCli.RevokeOpenApiCredential(l.ctx, &core.OpenApiCredentialIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformOpenApiCredentialResp{RespBase: platformlogic.PlatformRespBase(result.Base), Data: platformlogic.MapPlatformOpenApiCredential(result.Data)}, nil
}
