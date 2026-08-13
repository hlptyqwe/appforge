// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	corepb "appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPlatformChannelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformChannelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformChannelLogic {
	return &GetPlatformChannelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformChannelLogic) GetPlatformChannel(req *types.PlatformIdReq) (resp *types.PlatformChannelResp, err error) {
	item, err := l.svcCtx.CoreCli.GetChannel(l.ctx, &corepb.ChannelIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformChannelResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformChannel(item.Data)}, nil
}
