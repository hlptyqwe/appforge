package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RotateSourceBuildTriggerSecretLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRotateSourceBuildTriggerSecretLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotateSourceBuildTriggerSecretLogic {
	return &RotateSourceBuildTriggerSecretLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 轮换回调随机令牌和供应商签名Secret并一次性返回。
func (l *RotateSourceBuildTriggerSecretLogic) RotateSourceBuildTriggerSecret(in *core.SourceBuildTriggerIdReq) (*core.SourceBuildTriggerSecretResp, error) {
	return rotateSourceBuildTriggerSecret(l.ctx, l.svcCtx, in)
}
