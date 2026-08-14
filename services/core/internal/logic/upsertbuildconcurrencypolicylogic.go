package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertBuildConcurrencyPolicyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertBuildConcurrencyPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertBuildConcurrencyPolicyLogic {
	return &UpsertBuildConcurrencyPolicyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建或更新V4构建并发与公平调度策略。
func (l *UpsertBuildConcurrencyPolicyLogic) UpsertBuildConcurrencyPolicy(in *core.UpsertBuildConcurrencyPolicyReq) (*core.BuildConcurrencyPolicyResp, error) {
	return upsertBuildPolicy(l.ctx, l.svcCtx, in)
}
