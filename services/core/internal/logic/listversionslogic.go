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

type ListVersionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListVersionsLogic {
	return &ListVersionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListVersionsLogic) ListVersions(in *core.VersionListReq) (*core.VersionListResp, error) {
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
	if value := in.GetStatus(); value != core.VersionStatus_VERSION_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(value))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM t_app_version WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list versions count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TAppVersion
	query := fmt.Sprintf("SELECT id, tenant_id, app_id, version_code, version_name, source_apk_object_id, source_apk_url, source_apk_sha256, release_notes, build_config, status, published_at, create_by, create_time, update_time FROM t_app_version WHERE %s AND id > ? ORDER BY id ASC LIMIT ?", whereSQL)
	if err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &items, query, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list versions failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.Version, 0, len(items))
	var nextCursor int64
	for i := range items {
		data = append(data, mapVersion(&items[i]))
		nextCursor = items[i].Id
	}
	if !hasNext {
		nextCursor = 0
	}

	return &core.VersionListResp{Base: baseWithTotal(total, hasNext, nextCursor), Data: data}, nil
}
