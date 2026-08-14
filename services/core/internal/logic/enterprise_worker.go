package logic

import (
	"context"
	"fmt"
	"time"

	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// EnterpriseWorker converges Local Agent and certificate runtime state. It is
// independent from request traffic so disconnected customer nodes cannot stay
// falsely online.
type EnterpriseWorker struct {
	svcCtx *svc.ServiceContext
}

func NewEnterpriseWorker(svcCtx *svc.ServiceContext) *EnterpriseWorker {
	return &EnterpriseWorker{svcCtx: svcCtx}
}

func (w *EnterpriseWorker) Start(ctx context.Context) {
	if !w.svcCtx.Config.EnterpriseWorker.Enabled {
		return
	}
	interval := w.svcCtx.Config.EnterpriseWorker.PollInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if err := w.RunOnce(ctx); err != nil && ctx.Err() == nil {
				logx.WithContext(ctx).Errorf("enterprise state convergence failed: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (w *EnterpriseWorker) RunOnce(ctx context.Context) error {
	offlineAfter := w.svcCtx.Config.EnterpriseWorker.OfflineAfter
	if offlineAfter <= 0 {
		offlineAfter = 90 * time.Second
	}
	if _, err := w.svcCtx.DB.ExecCtx(ctx, `UPDATE t_local_agent_certificate
SET status=?,update_time=CURRENT_TIMESTAMP(3)
WHERE status=? AND not_after<=CURRENT_TIMESTAMP(3)`, localCertificateExpired, localCertificateActive); err != nil {
		return fmt.Errorf("expire Local Agent certificates: %w", err)
	}
	if _, err := w.svcCtx.DB.ExecCtx(ctx, `UPDATE t_local_agent
SET status=?,update_time=CURRENT_TIMESTAMP(3)
WHERE status=? AND (last_heartbeat_at IS NULL OR last_heartbeat_at<?)`, localAgentOffline, localAgentOnline, billingNow().Add(-offlineAfter)); err != nil {
		return fmt.Errorf("mark Local Agents offline: %w", err)
	}
	if _, err := w.svcCtx.DB.ExecCtx(ctx, `UPDATE t_local_agent a
SET a.drain_status=?,a.update_time=CURRENT_TIMESTAMP(3)
WHERE a.drain_status=? AND NOT EXISTS (
  SELECT 1 FROM t_build_slot_lease l
  WHERE l.node_code=CONCAT('local-',a.id) AND l.status=? AND l.lease_until>CURRENT_TIMESTAMP(3)
)`, int64(3), int64(2), buildSlotActive); err != nil {
		return fmt.Errorf("complete Local Agent drain: %w", err)
	}
	return nil
}
