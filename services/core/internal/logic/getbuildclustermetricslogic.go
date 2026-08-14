package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetBuildClusterMetricsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBuildClusterMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBuildClusterMetricsLogic {
	return &GetBuildClusterMetricsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询V4构建集群运行指标与告警摘要。
func (l *GetBuildClusterMetricsLogic) GetBuildClusterMetrics(in *core.BuildClusterMetricsReq) (*core.BuildClusterMetricsResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	poolCode, err := normalizedBuildPool(in.PoolCode)
	if err != nil {
		return nil, err
	}
	periodMinutes := in.PeriodMinutes
	if periodMinutes <= 0 {
		periodMinutes = 60
	}
	if periodMinutes > 7*24*60 {
		return nil, status.Error(codes.InvalidArgument, "period_minutes must not exceed 10080")
	}
	periodSeconds := int64(periodMinutes) * 60

	var nodeStats struct {
		OnlineNodes  int64 `db:"online_nodes"`
		OfflineNodes int64 `db:"offline_nodes"`
		Draining     int64 `db:"draining_nodes"`
		TotalSlots   int64 `db:"total_slots"`
		RunningSlots int64 `db:"running_slots"`
		DiskCapacity int64 `db:"disk_capacity"`
		DiskFree     int64 `db:"disk_free"`
		LowDiskNodes int64 `db:"low_disk_nodes"`
	}
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &nodeStats, `SELECT
COALESCE(SUM(CASE WHEN status=1 THEN 1 ELSE 0 END),0) AS online_nodes,
COALESCE(SUM(CASE WHEN status<>1 THEN 1 ELSE 0 END),0) AS offline_nodes,
COALESCE(SUM(CASE WHEN drain_status=2 THEN 1 ELSE 0 END),0) AS draining_nodes,
COALESCE(SUM(max_concurrency),0) AS total_slots,
COALESCE(SUM(running_count),0) AS running_slots,
COALESCE(SUM(disk_capacity),0) AS disk_capacity,
COALESCE(SUM(disk_free),0) AS disk_free,
COALESCE(SUM(CASE WHEN disk_capacity>0 AND disk_free*100<disk_capacity*10 THEN 1 ELSE 0 END),0) AS low_disk_nodes
FROM t_builder_node WHERE pool_code=?`, poolCode); err != nil {
		return nil, status.Errorf(codes.Internal, "get builder node metrics failed: %v", err)
	}

	var taskStats struct {
		Queued         int64 `db:"queued_tasks"`
		Running        int64 `db:"running_tasks"`
		Completed      int64 `db:"completed_tasks"`
		Success        int64 `db:"success_tasks"`
		Failed         int64 `db:"failed_tasks"`
		Cancelled      int64 `db:"cancelled_tasks"`
		AverageQueueMs int64 `db:"average_queue_ms"`
		AverageBuildMs int64 `db:"average_build_ms"`
		CacheHit       int64 `db:"cache_hit_tasks"`
		OldestQueuedMs int64 `db:"oldest_queued_ms"`
	}
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &taskStats, `SELECT
COALESCE(SUM(CASE WHEN status='PENDING' THEN 1 ELSE 0 END),0) AS queued_tasks,
COALESCE(SUM(CASE WHEN status IN ('BUILDING','SIGNING','UPLOADING') THEN 1 ELSE 0 END),0) AS running_tasks,
COALESCE(SUM(CASE WHEN status IN ('SUCCESS','FAILED','CANCELLED') AND COALESCE(finish_time,update_time)>=DATE_SUB(NOW(3),INTERVAL ? SECOND) THEN 1 ELSE 0 END),0) AS completed_tasks,
COALESCE(SUM(CASE WHEN status='SUCCESS' AND finish_time>=DATE_SUB(NOW(3),INTERVAL ? SECOND) THEN 1 ELSE 0 END),0) AS success_tasks,
COALESCE(SUM(CASE WHEN status='FAILED' AND finish_time>=DATE_SUB(NOW(3),INTERVAL ? SECOND) THEN 1 ELSE 0 END),0) AS failed_tasks,
COALESCE(SUM(CASE WHEN status='CANCELLED' AND COALESCE(finish_time,update_time)>=DATE_SUB(NOW(3),INTERVAL ? SECOND) THEN 1 ELSE 0 END),0) AS cancelled_tasks,
COALESCE(CAST(ROUND(AVG(CASE WHEN start_time>=DATE_SUB(NOW(3),INTERVAL ? SECOND) THEN TIMESTAMPDIFF(MICROSECOND,queued_at,start_time)/1000 END)) AS SIGNED),0) AS average_queue_ms,
COALESCE(CAST(ROUND(AVG(CASE WHEN finish_time>=DATE_SUB(NOW(3),INTERVAL ? SECOND) AND start_time IS NOT NULL THEN TIMESTAMPDIFF(MICROSECOND,start_time,finish_time)/1000 END)) AS SIGNED),0) AS average_build_ms,
COALESCE(SUM(CASE WHEN status='SUCCESS' AND finish_time>=DATE_SUB(NOW(3),INTERVAL ? SECOND) AND cache_hit=1 THEN 1 ELSE 0 END),0) AS cache_hit_tasks,
COALESCE(CAST(MAX(CASE WHEN status='PENDING' THEN TIMESTAMPDIFF(MICROSECOND,queued_at,NOW(3))/1000 ELSE 0 END) AS SIGNED),0) AS oldest_queued_ms
FROM t_build_task WHERE tenant_id=? AND pool_code=?`, periodSeconds, periodSeconds, periodSeconds, periodSeconds, periodSeconds, periodSeconds, periodSeconds, tenant, poolCode); err != nil {
		return nil, status.Errorf(codes.Internal, "get build task metrics failed: %v", err)
	}

	var cacheStats struct {
		Entries int64 `db:"active_cache_entries"`
		Bytes   int64 `db:"active_cache_bytes"`
	}
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &cacheStats, `SELECT COUNT(*) AS active_cache_entries,
COALESCE(SUM(size_bytes),0) AS active_cache_bytes FROM t_build_cache_entry
WHERE tenant_id=? AND status=? AND expires_at>NOW(3)`, tenant, buildCacheActive); err != nil {
		return nil, status.Errorf(codes.Internal, "get build cache metrics failed: %v", err)
	}

	var eventStats struct {
		LeaseRecovery          int64 `db:"lease_recovery_count"`
		CacheValidationFailure int64 `db:"cache_validation_failure_count"`
	}
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &eventStats, `SELECT
COALESCE(SUM(CASE WHEN reason_code='EXPIRED_LEASE_RECOVERY' THEN 1 ELSE 0 END),0) AS lease_recovery_count,
COALESCE(SUM(CASE WHEN reason_code='CACHE_DOWNLOAD_VALIDATION_FAILED' THEN 1 ELSE 0 END),0) AS cache_validation_failure_count
FROM t_build_scheduler_event WHERE tenant_id=? AND pool_code=? AND create_time>=DATE_SUB(NOW(3),INTERVAL ? SECOND)`, tenant, poolCode, periodSeconds); err != nil {
		return nil, status.Errorf(codes.Internal, "get build event metrics failed: %v", err)
	}

	alerts := make([]string, 0, 6)
	if nodeStats.OfflineNodes > 0 {
		alerts = append(alerts, "BUILDER_OFFLINE")
	}
	if nodeStats.LowDiskNodes > 0 {
		alerts = append(alerts, "BUILDER_LOW_DISK")
	}
	if taskStats.OldestQueuedMs > 5*60*1000 {
		alerts = append(alerts, "QUEUE_BACKLOG")
	}
	if taskStats.Failed > 0 {
		alerts = append(alerts, "RECENT_BUILD_FAILURE")
	}
	if eventStats.LeaseRecovery > 0 {
		alerts = append(alerts, "LEASE_RECOVERY")
	}
	if eventStats.CacheValidationFailure > 0 {
		alerts = append(alerts, "CACHE_VALIDATION_FAILURE")
	}

	successRate := float64(0)
	if terminal := taskStats.Success + taskStats.Failed; terminal > 0 {
		successRate = float64(taskStats.Success) / float64(terminal)
	}
	cacheHitRate := float64(0)
	if taskStats.Success > 0 {
		cacheHitRate = float64(taskStats.CacheHit) / float64(taskStats.Success)
	}

	return &core.BuildClusterMetricsResp{Base: okBase(), Data: &core.BuildClusterMetrics{
		PoolCode: poolCode, PeriodMinutes: periodMinutes,
		OnlineNodes: nodeStats.OnlineNodes, OfflineNodes: nodeStats.OfflineNodes,
		DrainingNodes: nodeStats.Draining, TotalSlots: nodeStats.TotalSlots,
		RunningSlots: nodeStats.RunningSlots, DiskCapacity: nodeStats.DiskCapacity, DiskFree: nodeStats.DiskFree,
		QueuedTasks: taskStats.Queued, RunningTasks: taskStats.Running, CompletedTasks: taskStats.Completed,
		SuccessTasks: taskStats.Success, FailedTasks: taskStats.Failed, CancelledTasks: taskStats.Cancelled,
		AverageQueueMs: taskStats.AverageQueueMs, AverageBuildMs: taskStats.AverageBuildMs,
		SuccessRate: successRate, CacheHitTasks: taskStats.CacheHit, CacheHitRate: cacheHitRate,
		ActiveCacheEntries: cacheStats.Entries, ActiveCacheBytes: cacheStats.Bytes,
		LeaseRecoveryCount:          eventStats.LeaseRecovery,
		CacheValidationFailureCount: eventStats.CacheValidationFailure,
		OldestQueuedMs:              taskStats.OldestQueuedMs, Alerts: alerts,
	}}, nil
}
