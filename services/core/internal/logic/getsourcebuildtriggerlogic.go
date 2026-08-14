package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSourceBuildTriggerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSourceBuildTriggerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSourceBuildTriggerLogic {
	return &GetSourceBuildTriggerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询预定义源码平台构建触发策略。
func (l *GetSourceBuildTriggerLogic) GetSourceBuildTrigger(in *core.SourceBuildTriggerIdReq) (*core.SourceBuildTriggerResp, error) {
	return getSourceBuildTrigger(l.ctx, l.svcCtx, in)
}
