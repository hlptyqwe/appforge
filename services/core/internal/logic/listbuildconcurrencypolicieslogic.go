package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListBuildConcurrencyPoliciesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBuildConcurrencyPoliciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBuildConcurrencyPoliciesLogic {
	return &ListBuildConcurrencyPoliciesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 分页查询V4构建并发与公平调度策略。
func (l *ListBuildConcurrencyPoliciesLogic) ListBuildConcurrencyPolicies(in *core.BuildConcurrencyPolicyListReq) (*core.BuildConcurrencyPolicyListResp, error) {
	return listBuildPolicies(l.ctx, l.svcCtx, in)
}
