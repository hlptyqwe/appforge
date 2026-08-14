package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateSourceBuildTriggerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateSourceBuildTriggerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSourceBuildTriggerLogic {
	return &CreateSourceBuildTriggerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建预定义源码平台构建触发策略并一次性返回回调令牌和签名Secret。
func (l *CreateSourceBuildTriggerLogic) CreateSourceBuildTrigger(in *core.CreateSourceBuildTriggerReq) (*core.SourceBuildTriggerSecretResp, error) {
	return createSourceBuildTrigger(l.ctx, l.svcCtx, in)
}
