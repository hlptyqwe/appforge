// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	"appforge/admin-api/internal/sourceoauth"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthorizePlatformSourceRepositoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthorizePlatformSourceRepositoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthorizePlatformSourceRepositoryLogic {
	return &AuthorizePlatformSourceRepositoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AuthorizePlatformSourceRepositoryLogic) AuthorizePlatformSourceRepository(req *types.AuthorizePlatformSourceRepositoryReq) (resp *types.PlatformSourceRepositoryResp, err error) {
	item, err := sourceoauth.AuthorizeRepository(l.ctx, l.svcCtx, req.Id, req.RepositoryId)
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceRepositoryResp{RespBase: types.RespBase{Code: item.Base.Code, Msg: item.Base.Msg},
		Data: mapPlatformSourceRepository(item.Data)}, nil
}
