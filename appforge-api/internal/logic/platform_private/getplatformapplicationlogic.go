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

type GetPlatformApplicationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformApplicationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformApplicationLogic {
	return &GetPlatformApplicationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformApplicationLogic) GetPlatformApplication(req *types.PlatformIdReq) (resp *types.PlatformApplicationResp, err error) {
	item, err := l.svcCtx.CoreCli.GetApplication(l.ctx, &core.ApplicationIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformApplicationResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformApplication(item.Data)}, nil
}
