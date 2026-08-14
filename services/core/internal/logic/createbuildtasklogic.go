package logic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

func buildInputDigest(input any) (string, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

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
	}{{in.AppId, "app_id"}, {in.VersionId, "version_id"}, {in.ChannelId, "channel_id"}} {
		if err := requirePositive(item.value, item.field); err != nil {
			return nil, err
		}
	}
	if in.WhiteLabelProductId <= 0 {
		if err := requirePositive(in.SigningConfigId, "signing_config_id"); err != nil {
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
	if version.SourceApkObjectId <= 0 {
		return nil, status.Error(codes.FailedPrecondition, "version has no validated source APK")
	}
	channel, err := l.svcCtx.ChannelModel.FindOne(l.ctx, in.ChannelId)
	if err != nil {
		return nil, notFoundOrInternal(err, "channel")
	}
	if channel.TenantId != tenant || channel.AppId != in.AppId || channel.Status != int64(core.ChannelStatus_CHANNEL_STATUS_ENABLED) {
		return nil, status.Error(codes.InvalidArgument, "channel is invalid")
	}
	product, whiteLabelDependency, templateSnapshot, err := prepareWhiteLabelBuildSnapshot(
		l.ctx, l.svcCtx, tenant, in.AppId, in.VersionId, in.WhiteLabelProductId, app.PackageName,
	)
	if err != nil {
		return nil, err
	}
	effectiveSigningConfigID := in.SigningConfigId
	effectiveBrandingProfileID := in.BrandingProfileId
	var templateRevision int64
	if product != nil {
		effectiveSigningConfigID = product.SigningConfigId
		effectiveBrandingProfileID = product.BrandingProfileId
		templateRevision = product.TemplateRevision
	}
	signingConfig, err := l.svcCtx.SigningConfigModel.FindOne(l.ctx, effectiveSigningConfigID)
	if err != nil {
		return nil, notFoundOrInternal(err, "signing config")
	}
	if signingConfig.TenantId != tenant || signingConfig.AppId != in.AppId || signingConfig.Status != 1 {
		return nil, status.Error(codes.InvalidArgument, "signing config is invalid")
	}
	if whiteLabelDependency != nil && signingConfig.Id != whiteLabelDependency.Signing.Id {
		return nil, status.Error(codes.FailedPrecondition, "white-label signing snapshot is inconsistent")
	}
	brandingRevision, brandingSnapshot, err := prepareBrandingSnapshot(l.ctx, l.svcCtx, tenant, in.AppId, in.VersionId, effectiveBrandingProfileID)
	if err != nil {
		return nil, err
	}
	priority := int64(in.Priority)
	if priority < 0 {
		priority = 0
	}
	poolCode, err := normalizedBuildPool(in.PoolCode)
	if err != nil {
		return nil, err
	}
	_, _, maxPriority, err := schedulerPolicy(l.ctx, l.svcCtx.DB, tenant, in.AppId, poolCode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load build policy failed: %v", err)
	}
	if priority > maxPriority {
		return nil, status.Errorf(codes.InvalidArgument, "priority exceeds policy maximum %d", maxPriority)
	}
	cacheKey, err := buildInputDigest(struct {
		TenantID            int64  `json:"tenantId"`
		AppID               int64  `json:"appId"`
		VersionID           int64  `json:"versionId"`
		SourceAPKObjectID   int64  `json:"sourceApkObjectId"`
		SourceAPKSHA256     string `json:"sourceApkSha256"`
		ChannelID           int64  `json:"channelId"`
		ChannelCode         string `json:"channelCode"`
		BuildConfig         string `json:"buildConfig"`
		BrandingRevision    int64  `json:"brandingRevision"`
		BrandingSnapshot    string `json:"brandingSnapshot"`
		WhiteLabelProductID int64  `json:"whiteLabelProductId"`
		TemplateRevision    int64  `json:"templateRevision"`
		TemplateSnapshot    string `json:"templateSnapshot"`
		SigningConfigID     int64  `json:"signingConfigId"`
		CertificateSHA256   string `json:"certificateSha256"`
	}{
		TenantID: tenant, AppID: in.AppId, VersionID: in.VersionId,
		SourceAPKObjectID: version.SourceApkObjectId, SourceAPKSHA256: stringValue(version.SourceApkSha256),
		ChannelID: in.ChannelId, ChannelCode: channel.ChannelCode, BuildConfig: stringValue(version.BuildConfig),
		BrandingRevision: brandingRevision, BrandingSnapshot: brandingSnapshot,
		WhiteLabelProductID: in.WhiteLabelProductId, TemplateRevision: templateRevision, TemplateSnapshot: templateSnapshot,
		SigningConfigID: effectiveSigningConfigID, CertificateSHA256: stringValue(signingConfig.CertificateSha256),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "compute build input digest failed: %v", err)
	}
	created := &models.TBuildTask{
		TenantId:             tenant,
		AppId:                in.AppId,
		VersionId:            in.VersionId,
		ChannelId:            in.ChannelId,
		SigningConfigId:      effectiveSigningConfigID,
		ChannelCode:          channel.ChannelCode,
		VersionCode:          version.VersionCode,
		VersionName:          version.VersionName,
		SourceApkObjectId:    version.SourceApkObjectId,
		SourceApkUrl:         version.SourceApkUrl,
		BuildConfig:          version.BuildConfig,
		BrandingProfileId:    effectiveBrandingProfileID,
		BrandingRevision:     brandingRevision,
		BrandingSnapshot:     nullString(brandingSnapshot),
		WhiteLabelProductId:  in.WhiteLabelProductId,
		TemplateRevision:     templateRevision,
		TemplateSnapshot:     nullString(templateSnapshot),
		PoolCode:             poolCode,
		CacheKey:             nullString(cacheKey),
		SourceWebhookEventId: sql.NullInt64{Int64: in.SourceWebhookEventId, Valid: in.SourceWebhookEventId > 0},
		Status:               buildStatusPending,
		BuilderAttempt:       0,
		Priority:             priority,
		QueuedAt:             timeFromMillis(0),
		CreateBy:             actorID(l.ctx),
	}
	var item models.TBuildTask
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(txCtx context.Context, session sqlx.Session) error {
		if in.SourceWebhookEventId > 0 {
			var eventAppID int64
			if err := session.QueryRowCtx(txCtx, &eventAppID, `SELECT t.app_id FROM t_source_webhook_event e
JOIN t_source_build_trigger t ON t.id = e.trigger_id AND t.tenant_id = e.tenant_id
WHERE e.id = ? AND e.tenant_id = ? AND e.status = 2 FOR UPDATE`, in.SourceWebhookEventId, tenant); err != nil || eventAppID != in.AppId {
				return status.Error(codes.FailedPrecondition, "source webhook event is not processing for this application")
			}
			if err := session.QueryRowCtx(txCtx, &item, buildTaskSelect+` WHERE source_webhook_event_id = ? AND channel_id = ? LIMIT 1`,
				in.SourceWebhookEventId, in.ChannelId); err == nil {
				if item.TenantId != tenant || item.AppId != in.AppId || item.VersionId != in.VersionId {
					return status.Error(codes.AlreadyExists, "source webhook channel build already exists with different input")
				}
				return nil
			} else if err != sqlx.ErrNotFound {
				return status.Errorf(codes.Internal, "find source webhook build task failed: %v", err)
			}
		}
		result, err := l.svcCtx.BuildTaskModel.WithSession(session).Insert(txCtx, created)
		if err != nil {
			return status.Errorf(codes.Internal, "create build task failed: %v", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return status.Errorf(codes.Internal, "read build task id failed: %v", err)
		}
		if _, err := reserveQuotaInSession(txCtx, session, tenant, "build.count", 1,
			"build", id, fmt.Sprintf("build:%d", id), 7*24*time.Hour); err != nil {
			return err
		}
		if product != nil {
			if err := recordPackageCertificateBuildTaskInTransaction(txCtx, l.svcCtx, session, tenant,
				product.PackageName, whiteLabelDependency.Signing, id); err != nil {
				return err
			}
		}
		if err := session.QueryRowCtx(txCtx, &item, buildTaskSelect+` WHERE id = ? AND tenant_id = ?`, id, tenant); err != nil {
			return status.Errorf(codes.Internal, "load created build task failed: %v", err)
		}
		if err := insertSchedulerEvent(txCtx, session, &item, "",
			core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_QUEUED, "TASK_CREATED",
			map[string]any{"priority": priority, "cacheInputDigest": cacheKey}); err != nil {
			return status.Errorf(codes.Internal, "record queued scheduler event failed: %v", err)
		}
		if _, _, err := insertOutboxEvent(txCtx, session, item.TenantId, "build.queued", "build", item.Id,
			map[string]any{"buildId": item.Id, "appId": item.AppId, "versionId": item.VersionId,
				"channelId": item.ChannelId, "priority": item.Priority}); err != nil {
			return status.Errorf(codes.Internal, "record queued webhook event failed: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &core.BuildTaskResp{Base: okBase(), Data: mapBuildTask(&item)}, nil
}

func recordPackageCertificateBuildTaskInTransaction(ctx context.Context, svcCtx *svc.ServiceContext, session sqlx.Session,
	tenant int64, packageName string, signing *models.TAppSigningConfig, taskID int64,
) error {
	var binding models.TPackageCertificateBinding
	if err := session.QueryRowCtx(ctx, &binding, `SELECT id, tenant_id, package_name, certificate_sha256,
signing_config_id, status, first_build_task_id, last_build_task_id, create_by, create_time, update_time
FROM t_package_certificate_binding WHERE tenant_id = ? AND package_name = ? FOR UPDATE`, tenant, packageName); err != nil {
		return status.Errorf(codes.Internal, "load package certificate binding failed: %v", err)
	}
	fingerprint := strings.ToLower(strings.TrimSpace(stringValue(signing.CertificateSha256)))
	if binding.Status != 1 || !strings.EqualFold(binding.CertificateSha256, fingerprint) || binding.SigningConfigId != signing.Id {
		return status.Error(codes.FailedPrecondition, "package certificate binding changed before build task creation")
	}
	if binding.FirstBuildTaskId == 0 {
		binding.FirstBuildTaskId = taskID
	}
	binding.LastBuildTaskId = taskID
	if err := svcCtx.PackageCertificateModel.WithSession(session).Update(ctx, &binding); err != nil {
		return status.Errorf(codes.Internal, "record package certificate build history failed: %v", err)
	}
	return nil
}
