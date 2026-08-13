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

type CreatePlatformChannelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePlatformChannelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlatformChannelLogic {
	return &CreatePlatformChannelLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePlatformChannelLogic) CreatePlatformChannel(req *types.CreatePlatformChannelReq) (resp *types.PlatformChannelResp, err error) {
	item, err := l.svcCtx.CoreCli.CreateChannel(l.ctx, &corepb.CreateChannelReq{AppId: req.AppId, ChannelCode: req.ChannelCode, ChannelName: req.ChannelName, LandingUrl: req.LandingUrl, DownloadUrl: req.DownloadUrl})
	if err != nil {
		return nil, err
	}
	return &types.PlatformChannelResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformChannel(item.Data)}, nil
}
