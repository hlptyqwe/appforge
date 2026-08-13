package platform_public

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	corepb "appforge/proto/core"

	"github.com/zeromicro/go-zero/core/logx"
)

type PlatformChannelEventLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPlatformChannelEventLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PlatformChannelEventLogic {
	return &PlatformChannelEventLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *PlatformChannelEventLogic) PlatformChannelEvent(req *types.PlatformChannelEventReq) (*types.RespBase, error) {
	item, err := l.svcCtx.CoreCli.ReportChannelEvent(l.ctx, &corepb.ReportChannelEventReq{
		AppId: req.AppId, ChannelCode: req.ChannelCode, EventType: req.EventType, EventKey: req.EventKey,
		InstallId: req.InstallId, UserId: req.UserId, AppVersion: req.AppVersion, Ip: req.Ip,
		DeviceModel: req.DeviceModel, EventTime: req.EventTime, Metadata: req.Metadata,
	})
	if err != nil {
		return nil, err
	}
	base := platformlogic.PlatformRespBase(item.Base)
	return &types.RespBase{Code: base.Code, Msg: base.Msg, Total: base.Total, HasNext: base.HasNext, HasPrev: base.HasPrev, NextCursor: base.NextCursor, PrevCursor: base.PrevCursor}, nil
}
