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

type ListPlatformBuildSchedulerEventsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformBuildSchedulerEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformBuildSchedulerEventsLogic {
	return &ListPlatformBuildSchedulerEventsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformBuildSchedulerEventsLogic) ListPlatformBuildSchedulerEvents(req *types.ListPlatformBuildSchedulerEventsReq) (resp *types.PlatformBuildSchedulerEventListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListBuildSchedulerEvents(l.ctx, &core.BuildSchedulerEventListReq{
		Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, TaskId: req.TaskId,
		NodeCode: req.NodeCode, PoolCode: req.PoolCode, EventType: core.BuildSchedulerEventType(req.EventType),
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformBuildSchedulerEvent, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformBuildSchedulerEvent(value))
	}
	return &types.PlatformBuildSchedulerEventListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
