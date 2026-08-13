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

type ListBuildTasksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBuildTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListBuildTasksLogic {
	return &ListBuildTasksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListBuildTasksLogic) ListBuildTasks(in *core.BuildTaskListReq) (*core.BuildTaskListResp, error) {
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
	if in.GetChannelId() > 0 {
		where = append(where, "channel_id = ?")
		args = append(args, in.GetChannelId())
	}
	if value := in.GetStatus(); value != core.BuildTaskStatus_BUILD_TASK_STATUS_UNKNOWN {
		statusValue, statusErr := protoStatusToDB(value)
		if statusErr != nil {
			return nil, statusErr
		}
		where = append(where, "status = ?")
		args = append(args, statusValue)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM t_build_task WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list build tasks count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TBuildTask
	query := fmt.Sprintf("SELECT id, tenant_id, app_id, version_id, channel_id, signing_config_id, channel_code, version_code, version_name, source_apk_url, build_config, status, builder_id, builder_attempt, priority, apk_url, apk_sha256, apk_size, log_url, error_message, queued_at, start_time, finish_time, lease_until, create_by, create_time, update_time FROM t_build_task WHERE %s AND id > ? ORDER BY priority DESC, id ASC LIMIT ?", whereSQL)
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &items, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list build tasks failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.BuildTask, 0, len(items))
	var nextCursor int64
	for i := range items {
		data = append(data, mapBuildTask(&items[i]))
		nextCursor = items[i].Id
	}
	if !hasNext {
		nextCursor = 0
	}

	return &core.BuildTaskListResp{Base: baseWithTotal(total, hasNext, nextCursor), Data: data}, nil
}
