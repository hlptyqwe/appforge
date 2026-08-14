package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBuildSchedulerEventsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBuildSchedulerEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBuildSchedulerEventsLogic {
	return &ListBuildSchedulerEventsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询V4结构化调度事件。
func (l *ListBuildSchedulerEventsLogic) ListBuildSchedulerEvents(in *core.BuildSchedulerEventListReq) (*core.BuildSchedulerEventListResp, error) {
	return listBuildSchedulerEvents(l.ctx, l.svcCtx, in)
}
