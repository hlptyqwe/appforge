package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveSourceBuildTriggerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveSourceBuildTriggerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveSourceBuildTriggerLogic {
	return &ResolveSourceBuildTriggerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 解析回调随机令牌和内部签名材料，仅供受内部认证保护的公开入口调用。
func (l *ResolveSourceBuildTriggerLogic) ResolveSourceBuildTrigger(in *core.ResolveSourceBuildTriggerReq) (*core.SourceBuildTriggerCredentialResp, error) {
	return resolveSourceBuildTrigger(l.ctx, l.svcCtx, in)
}
