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
	result, err := l.svcCtx.BuildTaskModel.Insert(l.ctx, &models.TBuildTask{
		TenantId:            tenant,
		AppId:               in.AppId,
		VersionId:           in.VersionId,
		ChannelId:           in.ChannelId,
		SigningConfigId:     effectiveSigningConfigID,
		ChannelCode:         channel.ChannelCode,
		VersionCode:         version.VersionCode,
		VersionName:         version.VersionName,
		SourceApkObjectId:   version.SourceApkObjectId,
		SourceApkUrl:        version.SourceApkUrl,
		BuildConfig:         version.BuildConfig,
		BrandingProfileId:   effectiveBrandingProfileID,
		BrandingRevision:    brandingRevision,
		BrandingSnapshot:    nullString(brandingSnapshot),
		WhiteLabelProductId: in.WhiteLabelProductId,
		TemplateRevision:    templateRevision,
		TemplateSnapshot:    nullString(templateSnapshot),
		Status:              buildStatusPending,
		BuilderAttempt:      0,
		Priority:            priority,
		QueuedAt:            timeFromMillis(0),
		CreateBy:            actorID(l.ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create build task failed: %v", err)
	}
	var item models.TBuildTask
	id, err := result.LastInsertId()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read build task id failed: %v", err)
	}
	if product != nil {
		if err := recordPackageCertificateBuildTask(l.ctx, l.svcCtx, tenant, product.PackageName, whiteLabelDependency.Signing, id); err != nil {
			if rollbackErr := l.svcCtx.BuildTaskModel.Delete(l.ctx, id); rollbackErr != nil {
				return nil, status.Errorf(codes.Internal, "record package certificate build history failed and rollback failed: %v; %v", err, rollbackErr)
			}
			return nil, err
		}
	}
	if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &item, buildTaskSelect+` WHERE id = ? AND tenant_id = ?`, id, tenant); err != nil {
		return nil, status.Errorf(codes.Internal, "load created build task failed: %v", err)
	}

	return &core.BuildTaskResp{Base: okBase(), Data: mapBuildTask(&item)}, nil
}
