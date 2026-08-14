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

type ListPlatformBuildConcurrencyPoliciesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPlatformBuildConcurrencyPoliciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPlatformBuildConcurrencyPoliciesLogic {
	return &ListPlatformBuildConcurrencyPoliciesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPlatformBuildConcurrencyPoliciesLogic) ListPlatformBuildConcurrencyPolicies(req *types.ListPlatformBuildConcurrencyPoliciesReq) (resp *types.PlatformBuildConcurrencyPolicyListResp, err error) {
	item, err := l.svcCtx.CoreCli.ListBuildConcurrencyPolicies(l.ctx, &core.BuildConcurrencyPolicyListReq{
		Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, PoolCode: req.PoolCode,
		Status: core.BuildPolicyStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformBuildConcurrencyPolicy, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformBuildConcurrencyPolicy(value))
	}
	return &types.PlatformBuildConcurrencyPolicyListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
