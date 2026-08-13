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

type ListPlatformChannelsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformChannelsLogic {
	return &ListPlatformChannelsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformChannelsLogic) ListPlatformChannels(req *types.ListPlatformChannelsReq) (resp *types.PlatformChannelListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListChannels(l.ctx, &corepb.ChannelListReq{Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, Keyword: req.Keyword, Status: corepb.ChannelStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformChannel, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformChannel(value))
	}
	return &types.PlatformChannelListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
