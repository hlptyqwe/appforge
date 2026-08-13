package logic

import (
	"context"
	"fmt"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListChannelsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListChannelsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListChannelsLogic {
	return &ListChannelsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListChannelsLogic) ListChannels(in *core.ChannelListReq) (*core.ChannelListResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	if in.GetAppId() > 0 {
		where = append(where, "app_id = ?")
		args = append(args, in.GetAppId())
	}
	if keyword := strings.TrimSpace(in.GetKeyword()); keyword != "" {
		where = append(where, "(channel_code LIKE ? OR channel_name LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	if value := in.GetStatus(); value != core.ChannelStatus_CHANNEL_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(value))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM t_promotion_channel WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list channels count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TPromotionChannel
	query := fmt.Sprintf("SELECT id, tenant_id, app_id, channel_code, channel_name, landing_url, download_url, status, create_by, create_time, update_time FROM t_promotion_channel WHERE %s AND id > ? ORDER BY id ASC LIMIT ?", whereSQL)
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &items, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list channels failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.Channel, 0, len(items))
	var nextCursor int64
	for i := range items {
		data = append(data, mapChannel(&items[i]))
		nextCursor = items[i].Id
	}
	if !hasNext {
		nextCursor = 0
	}

	return &core.ChannelListResp{Base: baseWithTotal(total, hasNext, nextCursor), Data: data}, nil
}
