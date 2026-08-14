package logic

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResolveChannelDownloadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveChannelDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveChannelDownloadLogic {
	return &ResolveChannelDownloadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 解析渠道最新成功产物并幂等记录点击、下载事件。
func (l *ResolveChannelDownloadLogic) ResolveChannelDownload(in *core.ResolveChannelDownloadReq) (*core.ChannelDownloadArtifactResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requireText(in.ChannelCode, "channel_code", 32); err != nil {
		return nil, err
	}
	if err := requireText(in.EventKey, "event_key", 96); err != nil {
		return nil, err
	}
	if err := requireOptionalText(in.Ip, "ip", 64); err != nil {
		return nil, err
	}
	if err := requireOptionalText(in.UserAgent, "user_agent", 500); err != nil {
		return nil, err
	}

	type artifactRow struct {
		TenantID     int64  `db:"tenant_id"`
		AppID        int64  `db:"app_id"`
		ChannelID    int64  `db:"channel_id"`
		ChannelCode  string `db:"channel_code"`
		BuildTaskID  int64  `db:"build_task_id"`
		ObjectID     int64  `db:"object_id"`
		ObjectKey    string `db:"object_key"`
		OriginalName string `db:"original_name"`
		ContentType  string `db:"content_type"`
	}
	var row artifactRow
	err := l.svcCtx.DB.QueryRowCtx(l.ctx, &row, `SELECT
c.tenant_id, c.app_id, c.id AS channel_id, c.channel_code,
b.id AS build_task_id, o.id AS object_id, o.object_key, o.original_name, o.content_type
FROM t_promotion_channel c
JOIN t_build_task b ON b.channel_id = c.id AND b.tenant_id = c.tenant_id
JOIN t_storage_object o ON o.id = b.apk_object_id AND o.tenant_id = c.tenant_id
WHERE c.channel_code = ? AND c.status = ? AND b.status = ?
  AND b.apk_object_id > 0 AND o.object_type = ? AND o.status = ?
ORDER BY b.finish_time DESC, b.id DESC LIMIT 1`,
		strings.TrimSpace(in.ChannelCode), int64(core.ChannelStatus_CHANNEL_STATUS_ENABLED), buildStatusSuccess,
		int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILT_APK), storageStatusBound)
	if err != nil {
		if err == models.ErrNotFound || err == sqlx.ErrNotFound {
			return nil, status.Error(codes.NotFound, "channel has no downloadable build")
		}
		return nil, status.Errorf(codes.Internal, "resolve channel download failed: %v", err)
	}

	metadata, _ := json.Marshal(map[string]string{"userAgent": strings.TrimSpace(in.UserAgent)})
	baseKey := strings.TrimSpace(in.EventKey)
	for _, event := range []struct {
		typeID int64
		suffix string
	}{
		{typeID: eventTypeClick, suffix: "click"},
		{typeID: eventTypeDownload, suffix: "download"},
	} {
		eventKey := baseKey + ":" + event.suffix
		if _, findErr := l.svcCtx.ChannelEventModel.FindOneByTenantIdEventTypeEventKey(l.ctx, row.TenantID, event.typeID, eventKey); findErr == nil {
			continue
		} else if findErr != models.ErrNotFound {
			return nil, status.Errorf(codes.Internal, "check download event failed: %v", findErr)
		}
		if _, insertErr := l.svcCtx.ChannelEventModel.Insert(l.ctx, &models.TChannelEvent{
			TenantId: row.TenantID, AppId: row.AppID, ChannelId: row.ChannelID, ChannelCode: row.ChannelCode,
			EventType: event.typeID, EventKey: eventKey, Ip: nullString(in.Ip),
			EventTime: time.Now(), Metadata: nullString(string(metadata)),
		}); insertErr != nil {
			// 唯一键竞争说明同一匿名访问已由并发请求记录，可安全视为成功。
			if _, findErr := l.svcCtx.ChannelEventModel.FindOneByTenantIdEventTypeEventKey(l.ctx, row.TenantID, event.typeID, eventKey); findErr != nil {
				return nil, status.Errorf(codes.Internal, "record download event failed: %v", insertErr)
			}
		}
	}

	return &core.ChannelDownloadArtifactResp{
		Base: okBase(),
		Data: &core.ChannelDownloadArtifact{
			TenantId: row.TenantID, AppId: row.AppID, ChannelId: row.ChannelID,
			ChannelCode: row.ChannelCode, BuildTaskId: row.BuildTaskID, ObjectId: row.ObjectID,
			ObjectKey: row.ObjectKey, OriginalName: row.OriginalName, ContentType: row.ContentType,
		},
	}, nil
}
