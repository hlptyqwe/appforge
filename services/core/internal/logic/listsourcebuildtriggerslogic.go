package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSourceBuildTriggersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSourceBuildTriggersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSourceBuildTriggersLogic {
	return &ListSourceBuildTriggersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询预定义源码平台构建触发策略。
func (l *ListSourceBuildTriggersLogic) ListSourceBuildTriggers(in *core.SourceBuildTriggerListReq) (*core.SourceBuildTriggerListResp, error) {
	return listSourceBuildTriggers(l.ctx, l.svcCtx, in)
}
