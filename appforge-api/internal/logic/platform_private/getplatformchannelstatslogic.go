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

type GetPlatformChannelStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformChannelStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformChannelStatsLogic {
	return &GetPlatformChannelStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformChannelStatsLogic) GetPlatformChannelStats(req *types.GetPlatformChannelStatsReq) (resp *types.PlatformChannelStatsResp, err error) {
	item, err := l.svcCtx.CoreCli.GetChannelStats(l.ctx, &corepb.ChannelStatsReq{AppId: req.AppId, ChannelId: req.ChannelId, StartTime: req.StartTime, EndTime: req.EndTime})
	if err != nil {
		return nil, err
	}
	return &types.PlatformChannelStatsResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformStats(item.Data)}, nil
}
