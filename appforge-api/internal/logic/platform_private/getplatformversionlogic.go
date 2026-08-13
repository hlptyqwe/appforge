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

type GetPlatformVersionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformVersionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformVersionLogic {
	return &GetPlatformVersionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformVersionLogic) GetPlatformVersion(req *types.PlatformIdReq) (resp *types.PlatformVersionResp, err error) {
	item, err := l.svcCtx.CoreCli.GetVersion(l.ctx, &core.VersionIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformVersionResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformVersion(item.Data)}, nil
}
