package tasks

import (
	"context"
	"fmt"
	"time"
	mq "appforge/common/mq/kafka"
)

const (
	channel = "system:scheduled-tasks"

	ServiceMarket    = "market"
	ServiceLiquidity = "liquidity"
	ServiceOption    = "option"
	ServiceStaking   = "staking"
	ServiceTrade     = "trade"

	ActionMarketSyncProducts = "SyncProducts"
	ActionMarketSyncKlines   = "SyncKlines"

	ActionOptionProcessContractLifecycle   = "ProcessContractLifecycle"
	ActionOptionProcessCorporateActions    = "ProcessCorporateActions"
	ActionOptionCleanMarketSnapshots       = "CleanMarketSnapshots"
	ActionOptionProcessAssetInstructions   = "ProcessAssetInstructions"
	ActionOptionProcessTradeEvents         = "ProcessTradeEvents"
	ActionOptionProcessRiskAccounts        = "ProcessRiskAccounts"
	ActionOptionProcessLiquidations        = "ProcessLiquidations"
	ActionOptionProcessExercises           = "ProcessExercises"
	ActionOptionProcessDailyReconciliation = "ProcessDailyReconciliation"

	ActionStakingProcessRewardsAndSettleOrders = "ProcessRewardsAndSettleOrders"
	ActionStakingReconcile                     = "ReconcileStaking"

	ActionTradeProcessOrderMatching       = "ProcessOrderMatching"
	ActionTradeProcessPositions           = "ProcessPositions"
	ActionTradeProcessContractSettlements = "ProcessContractSettlements"
	ActionTradeProcessSecondsSettlements  = "ProcessSecondsSettlements"
	ActionTradeProcessTradeEvents         = "ProcessTradeEvents"
	ActionTradeExpireRiskLimits           = "ExpireRiskLimits"
	ActionTradeArchiveLiquidityOrders     = "ArchiveLiquidityOrders"

	ActionLiquidityRefreshQuotes      = "RefreshQuotes"
	ActionLiquidityRecoverQuoteOrders = "RecoverQuoteOrders"
)

type Message struct {
	ID        string `json:"id"`
	Service   string `json:"service"`
	Action    string `json:"action"`
	TenantID  int64  `json:"tenantId,omitempty"`
	JobID     int64  `json:"jobId,omitempty"`
	JobName   string `json:"jobName,omitempty"`
	CreatedAt int64  `json:"createdAt"`
}

type PublishOptions struct {
	TenantID int64
	JobID    int64
	JobName  string
}

type Handler func(context.Context, Message) error

func NewMessage(service, action string, tenantID int64, jobID int64, jobName string) Message {
	now := time.Now().UnixMilli()
	return Message{
		ID:        fmt.Sprintf("%d", now),
		Service:   service,
		Action:    action,
		TenantID:  tenantID,
		JobID:     jobID,
		JobName:   jobName,
		CreatedAt: now,
	}
}

func Publish(ctx context.Context, publisher *mq.Publisher, service string, action string, opts PublishOptions) error {
	if publisher == nil {
		return fmt.Errorf("task publisher is nil")
	}

	msg := NewMessage(service, action, opts.TenantID, opts.JobID, opts.JobName)
	return publisher.Publish(ctx, channel, msg)
}

func SubscribeService(ctx context.Context, subscriber *mq.Subscriber, service string, handler Handler) error {
	if subscriber == nil {
		return fmt.Errorf("task subscriber is nil")
	}
	if service == "" {
		return fmt.Errorf("task service is empty")
	}
	if handler == nil {
		return fmt.Errorf("task handler is nil")
	}

	return subscriber.Subscribe(ctx, channel, func(ctx context.Context, msg mq.Message) error {
		var payload Message
		if err := mq.Decode(msg, &payload); err != nil {
			return err
		}
		if payload.Service != service {
			return nil
		}
		return handler(ctx, payload)
	})
}
