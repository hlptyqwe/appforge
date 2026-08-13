package logic

import (
	"context"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateBuildTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateBuildTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateBuildTaskLogic {
	return &CreateBuildTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateBuildTaskLogic) CreateBuildTask(in *core.CreateBuildTaskReq) (*core.BuildTaskResp, error) {
	tenant, err := tenantID(l.ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	for _, item := range []struct {
		value int64
		field string
	}{{in.AppId, "app_id"}, {in.VersionId, "version_id"}, {in.ChannelId, "channel_id"}, {in.SigningConfigId, "signing_config_id"}} {
		if err := requirePositive(item.value, item.field); err != nil {
			return nil, err
		}
	}
	app, err := l.svcCtx.ApplicationModel.FindOne(l.ctx, in.AppId)
	if err != nil {
		return nil, notFoundOrInternal(err, "application")
	}
	if err := ensureTenant(app.TenantId, tenant); err != nil {
		return nil, err
	}
	version, err := l.svcCtx.VersionModel.FindOne(l.ctx, in.VersionId)
	if err != nil {
		return nil, notFoundOrInternal(err, "version")
	}
	if version.TenantId != tenant || version.AppId != in.AppId {
		return nil, status.Error(codes.NotFound, "version not found")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.ChannelId)
	if err != nil {
		return nil, notFoundOrInternal(err, "channel")
	}
	if channel.TenantId != tenant || channel.AppId != in.AppId || channel.Status != int64(core.ChannelStatus_CHANNEL_STATUS_ENABLED) {
		return nil, status.Error(codes.InvalidArgument, "channel is invalid")
	}
	signingConfig, err := l.svcCtx.SigningConfigModel.FindOne(l.ctx, in.SigningConfigId)
	if err != nil {
		return nil, notFoundOrInternal(err, "signing config")
	}
	if signingConfig.TenantId != tenant || signingConfig.AppId != in.AppId || signingConfig.Status != 1 {
		return nil, status.Error(codes.InvalidArgument, "signing config is invalid")
	}
	priority := int64(in.Priority)
	if priority < 0 {
		priority = 0
	}
	result, err := l.svcCtx.BuildTaskModel.Insert(l.ctx, &models.TBuildTask{
		TenantId:        tenant,
		AppId:           in.AppId,
		VersionId:       in.VersionId,
		ChannelId:       in.ChannelId,
		SigningConfigId: in.SigningConfigId,
		ChannelCode:     channel.ChannelCode,
		VersionCode:     version.VersionCode,
		VersionName:     version.VersionName,
		SourceApkUrl:    version.SourceApkUrl,
		BuildConfig:     version.BuildConfig,
		Status:          buildStatusPending,
		BuilderAttempt:  0,
		Priority:        priority,
		QueuedAt:        timeFromMillis(0),
		CreateBy:        actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create build task failed: %v", err)
	}
	var item models.TBuildTask
	id, err := result.LastInsertId()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read build task id failed: %v", err)
	}
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &item, `SELECT id, tenant_id, app_id, version_id, channel_id, signing_config_id, channel_code, version_code, version_name, source_apk_url, build_config, status, builder_id, builder_attempt, priority, apk_url, apk_sha256, apk_size, log_url, error_message, queued_at, start_time, finish_time, lease_until, create_by, create_time, update_time FROM t_build_task WHERE id = ? AND tenant_id = ?`, id, tenant); err != nil {
		return nil, status.Errorf(codes.Internal, "load created build task failed: %v", err)
	}

	return &core.BuildTaskResp{Base: okBase(), Data: mapBuildTask(&item)}, nil
}
