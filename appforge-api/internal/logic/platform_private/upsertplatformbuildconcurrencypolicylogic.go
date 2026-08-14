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

type UpsertPlatformBuildConcurrencyPolicyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpsertPlatformBuildConcurrencyPolicyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertPlatformBuildConcurrencyPolicyLogic {
	return &UpsertPlatformBuildConcurrencyPolicyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpsertPlatformBuildConcurrencyPolicyLogic) UpsertPlatformBuildConcurrencyPolicy(req *types.UpsertPlatformBuildConcurrencyPolicyReq) (resp *types.PlatformBuildConcurrencyPolicyResp, err error) {
	item, err := l.svcCtx.CoreCli.UpsertBuildConcurrencyPolicy(l.ctx, &core.UpsertBuildConcurrencyPolicyReq{
		Id: req.Id, AppId: req.AppId, PoolCode: req.PoolCode, MaxConcurrency: req.MaxConcurrency,
		FairWeight: req.FairWeight, MaxPriority: req.MaxPriority, Status: core.BuildPolicyStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBuildConcurrencyPolicyResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBuildConcurrencyPolicy(item.Data)}, nil
}
