package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSourceBuildTriggerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSourceBuildTriggerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSourceBuildTriggerLogic {
	return &UpdateSourceBuildTriggerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 更新预定义源码平台构建触发策略，不能修改授权仓库和应用。
func (l *UpdateSourceBuildTriggerLogic) UpdateSourceBuildTrigger(in *core.UpdateSourceBuildTriggerReq) (*core.SourceBuildTriggerResp, error) {
	return updateSourceBuildTrigger(l.ctx, l.svcCtx, in)
}
