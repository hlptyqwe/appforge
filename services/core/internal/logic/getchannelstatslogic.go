package logic

import (
	"context"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetChannelStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetChannelStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetChannelStatsLogic {
	return &GetChannelStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetChannelStatsLogic) GetChannelStats(in *core.ChannelStatsReq) (*core.ChannelStatsResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.AppId, "app_id"); err != nil {
		return nil, err
	}
	if err := requirePositive(in.ChannelId, "channel_id"); err != nil {
		return nil, err
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.ChannelId)
	if err != nil {
		return nil, notFoundOrInternal(err, "channel")
	}
	if channel.TenantId != tenant || channel.AppId != in.AppId {
		return nil, status.Error(codes.NotFound, "channel not found")
	}
	start := time.Unix(0, 0)
	if in.StartTime > 0 {
		start = time.UnixMilli(in.StartTime)
	}
	end := time.Now()
	if in.EndTime > 0 {
		end = time.UnixMilli(in.EndTime)
	}
	if in.EndTime <= 0 {
		end = time.Now()
	}
	if end.Before(start) {
		return nil, status.Error(codes.InvalidArgument, "end_time must not be before start_time")
	}
	var result struct {
		Clicks        int64 `db:"clicks"`
		Downloads     int64 `db:"downloads"`
		Installs      int64 `db:"installs"`
		Registrations int64 `db:"registrations"`
		FirstPays     int64 `db:"first_pays"`
		Pays          int64 `db:"pays"`
	}
	query := `SELECT
COALESCE(SUM(CASE WHEN event_type = 1 THEN 1 ELSE 0 END), 0) AS clicks,
COALESCE(SUM(CASE WHEN event_type = 2 THEN 1 ELSE 0 END), 0) AS downloads,
COALESCE(SUM(CASE WHEN event_type = 3 THEN 1 ELSE 0 END), 0) AS installs,
COALESCE(SUM(CASE WHEN event_type = 4 THEN 1 ELSE 0 END), 0) AS registrations,
COALESCE(SUM(CASE WHEN event_type = 5 THEN 1 ELSE 0 END), 0) AS first_pays,
COALESCE(SUM(CASE WHEN event_type = 6 THEN 1 ELSE 0 END), 0) AS pays
FROM t_channel_event
WHERE tenant_id = ? AND app_id = ? AND channel_id = ? AND event_time >= ? AND event_time <= ?`
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &result, query, tenant, in.AppId, in.ChannelId, start, end); err != nil {
		return nil, status.Errorf(codes.Internal, "get channel stats failed: %v", err)
	}

	return &core.ChannelStatsResp{
		Base: okBase(),
		Data: &core.ChannelStats{
			ChannelId:     channel.Id,
			ChannelCode:   channel.ChannelCode,
			Clicks:        result.Clicks,
			Downloads:     result.Downloads,
			Installs:      result.Installs,
			Registrations: result.Registrations,
			FirstPays:     result.FirstPays,
			Pays:          result.Pays,
		},
	}, nil
}
