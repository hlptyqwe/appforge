// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/sourceoauth"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePlatformSourceOAuthAuthorizationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformSourceOAuthAuthorizationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformSourceOAuthAuthorizationLogic {
	return &CreatePlatformSourceOAuthAuthorizationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformSourceOAuthAuthorizationLogic) CreatePlatformSourceOAuthAuthorization(req *types.CreatePlatformSourceOAuthAuthorizationReq) (resp *types.PlatformSourceOAuthAuthorizationResp, err error) {
	authorizationURL, err := sourceoauth.Begin(l.ctx, l.svcCtx, core.SourcePlatform(req.Platform))
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceOAuthAuthorizationResp{RespBase: types.RespBase{Code: 200, Msg: "OK"},
		Data: types.PlatformSourceOAuthAuthorization{AuthorizationUrl: authorizationURL}}, nil
}
