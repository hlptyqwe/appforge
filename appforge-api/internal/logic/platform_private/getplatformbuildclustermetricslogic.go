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

type GetPlatformBuildClusterMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPlatformBuildClusterMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPlatformBuildClusterMetricsLogic {
	return &GetPlatformBuildClusterMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPlatformBuildClusterMetricsLogic) GetPlatformBuildClusterMetrics(req *types.GetPlatformBuildClusterMetricsReq) (resp *types.PlatformBuildClusterMetricsResp, err error) {
	result, err := l.svcCtx.CoreCli.GetBuildClusterMetrics(l.ctx, &core.BuildClusterMetricsReq{
		PoolCode: req.PoolCode, PeriodMinutes: req.PeriodMinutes,
	})
	if err != nil {
		return nil, err
	}
	data := result.Data
	if data == nil {
		data = &core.BuildClusterMetrics{}
	}
	return &types.PlatformBuildClusterMetricsResp{
		RespBase: platformlogic.PlatformRespBase(result.Base),
		Data: types.PlatformBuildClusterMetrics{
			PoolCode: data.PoolCode, PeriodMinutes: data.PeriodMinutes,
			OnlineNodes: data.OnlineNodes, OfflineNodes: data.OfflineNodes,
			DrainingNodes: data.DrainingNodes, TotalSlots: data.TotalSlots,
			RunningSlots: data.RunningSlots, DiskCapacity: data.DiskCapacity, DiskFree: data.DiskFree,
			QueuedTasks: data.QueuedTasks, RunningTasks: data.RunningTasks,
			CompletedTasks: data.CompletedTasks, SuccessTasks: data.SuccessTasks,
			FailedTasks: data.FailedTasks, CancelledTasks: data.CancelledTasks,
			AverageQueueMs: data.AverageQueueMs, AverageBuildMs: data.AverageBuildMs,
			SuccessRate: data.SuccessRate, CacheHitTasks: data.CacheHitTasks,
			CacheHitRate: data.CacheHitRate, ActiveCacheEntries: data.ActiveCacheEntries,
			ActiveCacheBytes: data.ActiveCacheBytes, LeaseRecoveryCount: data.LeaseRecoveryCount,
			CacheValidationFailureCount: data.CacheValidationFailureCount,
			OldestQueuedMs:              data.OldestQueuedMs, Alerts: data.Alerts,
		},
	}, nil
}
