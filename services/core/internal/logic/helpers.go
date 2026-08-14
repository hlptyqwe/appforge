package logic

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"appforge/common/utils"
	"appforge/proto/common"
	"appforge/proto/core"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	buildStatusPending   = "PENDING"
	buildStatusBuilding  = "BUILDING"
	buildStatusSigning   = "SIGNING"
	buildStatusUploading = "UPLOADING"
	buildStatusSuccess   = "SUCCESS"
	buildStatusFailed    = "FAILED"
	buildStatusCancelled = "CANCELLED"

	eventTypeClick    = 1
	eventTypeDownload = 2
	eventTypeInstall  = 3
	eventTypeRegister = 4
	eventTypeFirstPay = 5
	eventTypePay      = 6

	storageStatusUploading = int64(core.StorageObjectStatus_STORAGE_OBJECT_STATUS_UPLOADING)
	storageStatusReady     = int64(core.StorageObjectStatus_STORAGE_OBJECT_STATUS_READY)
	storageStatusBound     = int64(core.StorageObjectStatus_STORAGE_OBJECT_STATUS_BOUND)
	storageStatusDeleted   = int64(core.StorageObjectStatus_STORAGE_OBJECT_STATUS_DELETED)
	storageStatusFailed    = int64(core.StorageObjectStatus_STORAGE_OBJECT_STATUS_FAILED)
)

func tenantID(ctx context.Context) (int64, error) {
	if id, err := utils.GetTrustedTenantIdFromCtx(ctx); err == nil && id > 0 {
		return id, nil
	}
	if id, err := utils.GetTenantIdFromMd(ctx); err == nil && id > 0 {
		return id, nil
	}
	return 0, status.Error(codes.InvalidArgument, "tenant context is required")
}

func actorID(ctx context.Context) int64 {
	if id, err := utils.GetUserIdFromCtx(ctx); err == nil && id > 0 {
		return id
	}
	if id, err := utils.GetUserIdFromMd(ctx); err == nil && id > 0 {
		return id
	}
	return 0
}

func requireText(value, field string, max int) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return status.Errorf(codes.InvalidArgument, "%s is required", field)
	}
	if len(value) > max {
		return status.Errorf(codes.InvalidArgument, "%s is too long", field)
	}
	return nil
}

func requireOptionalText(value, field string, max int) error {
	if len(strings.TrimSpace(value)) > max {
		return status.Errorf(codes.InvalidArgument, "%s is too long", field)
	}
	return nil
}

func requirePositive(value int64, field string) error {
	if value <= 0 {
		return status.Errorf(codes.InvalidArgument, "%s must be greater than zero", field)
	}
	return nil
}

func newChannelCode() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("ch%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("ch%x", buf)
}

func okBase() *common.RespBase {
	return &common.RespBase{Code: 200, Msg: "OK"}
}

func baseWithTotal(total int64, hasNext bool, nextCursor int64) *common.RespBase {
	return &common.RespBase{Code: 200, Msg: "OK", Total: total, HasNext: hasNext, NextCursor: nextCursor}
}

func timeFromMillis(value int64) time.Time {
	if value <= 0 {
		return time.Now()
	}
	return time.UnixMilli(value)
}

func millis(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: strings.TrimSpace(value), Valid: strings.TrimSpace(value) != ""}
}

func nullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: value > 0}
}

func nullTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: !value.IsZero()}
}

func stringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func int64Value(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func timeValue(value sql.NullTime) int64 {
	if !value.Valid {
		return 0
	}
	return millis(value.Time)
}

func mapApplication(item *models.TAppApplication) *core.Application {
	if item == nil {
		return nil
	}
	return &core.Application{
		Id:          item.Id,
		TenantId:    item.TenantId,
		AppCode:     item.AppCode,
		AppName:     item.AppName,
		PackageName: item.PackageName,
		Description: stringValue(item.Description),
		IconUrl:     stringValue(item.IconUrl),
		ApiHost:     stringValue(item.ApiHost),
		Status:      core.ApplicationStatus(item.Status),
		CreateBy:    item.CreateBy,
		CreateTime:  millis(item.CreateTime),
		UpdateTime:  millis(item.UpdateTime),
	}
}

func mapVersion(item *models.TAppVersion) *core.Version {
	if item == nil {
		return nil
	}
	return &core.Version{
		Id:                item.Id,
		TenantId:          item.TenantId,
		AppId:             item.AppId,
		VersionCode:       item.VersionCode,
		VersionName:       item.VersionName,
		SourceApkUrl:      stringValue(item.SourceApkUrl),
		SourceApkSha256:   stringValue(item.SourceApkSha256),
		ReleaseNotes:      stringValue(item.ReleaseNotes),
		BuildConfigJson:   stringValue(item.BuildConfig),
		Status:            core.VersionStatus(item.Status),
		PublishedAt:       timeValue(item.PublishedAt),
		CreateBy:          item.CreateBy,
		CreateTime:        millis(item.CreateTime),
		UpdateTime:        millis(item.UpdateTime),
		SourceApkObjectId: item.SourceApkObjectId,
	}
}

func mapChannel(item *models.TPromotionChannel) *core.Channel {
	if item == nil {
		return nil
	}
	return &core.Channel{
		Id:          item.Id,
		TenantId:    item.TenantId,
		AppId:       item.AppId,
		ChannelCode: item.ChannelCode,
		ChannelName: item.ChannelName,
		LandingUrl:  stringValue(item.LandingUrl),
		DownloadUrl: stringValue(item.DownloadUrl),
		Status:      core.ChannelStatus(item.Status),
		CreateBy:    item.CreateBy,
		CreateTime:  millis(item.CreateTime),
		UpdateTime:  millis(item.UpdateTime),
	}
}

func mapSigningConfig(item *models.TAppSigningConfig) *core.SigningConfig {
	if item == nil {
		return nil
	}
	return &core.SigningConfig{
		Id:                item.Id,
		TenantId:          item.TenantId,
		AppId:             item.AppId,
		Name:              item.Name,
		KeystoreObjectKey: item.KeystoreObjectKey,
		KeyAlias:          item.KeyAlias,
		SecretRef:         stringValue(item.SecretRef),
		Status:            item.Status,
		LastVerifiedAt:    timeValue(item.LastVerifiedAt),
		CreateBy:          item.CreateBy,
		CreateTime:        millis(item.CreateTime),
		UpdateTime:        millis(item.UpdateTime),
		KeystoreObjectId:  item.KeystoreObjectId,
		CertificateSha256: stringValue(item.CertificateSha256),
	}
}

func mapBuildTask(item *models.TBuildTask) *core.BuildTask {
	if item == nil {
		return nil
	}
	return &core.BuildTask{
		Id:                   item.Id,
		TenantId:             item.TenantId,
		AppId:                item.AppId,
		VersionId:            item.VersionId,
		ChannelId:            item.ChannelId,
		SigningConfigId:      item.SigningConfigId,
		ChannelCode:          item.ChannelCode,
		VersionCode:          item.VersionCode,
		VersionName:          item.VersionName,
		Status:               buildStatusToProto(item.Status),
		BuilderId:            stringValue(item.BuilderId),
		BuilderAttempt:       int32(item.BuilderAttempt),
		Priority:             int32(item.Priority),
		ApkUrl:               stringValue(item.ApkUrl),
		ApkSha256:            stringValue(item.ApkSha256),
		ApkSize:              item.ApkSize,
		LogUrl:               stringValue(item.LogUrl),
		ErrorMessage:         stringValue(item.ErrorMessage),
		QueuedAt:             millis(item.QueuedAt),
		StartTime:            timeValue(item.StartTime),
		FinishTime:           timeValue(item.FinishTime),
		CreateBy:             item.CreateBy,
		CreateTime:           millis(item.CreateTime),
		UpdateTime:           millis(item.UpdateTime),
		SourceApkUrl:         stringValue(item.SourceApkUrl),
		BuildConfigJson:      stringValue(item.BuildConfig),
		SourceApkObjectId:    item.SourceApkObjectId,
		ApkObjectId:          item.ApkObjectId,
		LogObjectId:          item.LogObjectId,
		BrandingProfileId:    item.BrandingProfileId,
		BrandingRevision:     int32(item.BrandingRevision),
		BrandingSnapshotJson: stringValue(item.BrandingSnapshot),
		WhiteLabelProductId:  item.WhiteLabelProductId,
		TemplateRevision:     int32(item.TemplateRevision),
		TemplateSnapshotJson: stringValue(item.TemplateSnapshot),
		PoolCode:             item.PoolCode,
		CacheKey:             stringValue(item.CacheKey),
		CacheEntryId:         item.CacheEntryId,
		CacheHit:             item.CacheHit == 1,
		CancelRequestedAt:    timeValue(item.CancelRequestedAt),
		CancelledAt:          timeValue(item.CancelledAt),
		CancelReason:         stringValue(item.CancelReason),
		RetryOfTaskId:        item.RetryOfTaskId,
	}
}

func mapBrandingProfile(item *models.TAppBrandingProfile) *core.BrandingProfile {
	if item == nil {
		return nil
	}
	return &core.BrandingProfile{
		Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		ProfileName: item.ProfileName, AppName: item.AppName,
		LogoObjectId: item.LogoObjectId, SplashObjectId: item.SplashObjectId,
		ApiHost: item.ApiHost, RewriteMode: core.BrandingRewriteMode(item.RewriteMode),
		LauncherIconTarget:   stringValue(item.LauncherIconTarget),
		SplashResourceTarget: stringValue(item.SplashResourceTarget),
		RuntimeConfigJson:    stringValue(item.RuntimeConfig),
		Status:               core.BrandingProfileStatus(item.Status), Revision: int32(item.Revision),
		CreateBy: item.CreateBy, CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func mapBrandingPreflight(item *models.TBrandingPreflight) *core.BrandingPreflight {
	if item == nil {
		return nil
	}
	return &core.BrandingPreflight{
		Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		BrandingProfileId: item.BrandingProfileId, BrandingRevision: int32(item.BrandingRevision),
		VersionId: item.VersionId, Status: core.BrandingPreflightStatus(item.Status),
		ReportJson: stringValue(item.ReportJson), SourceApkSha256: stringValue(item.SourceApkSha256),
		ToolchainVersion: stringValue(item.ToolchainVersion), StartTime: timeValue(item.StartTime),
		FinishTime: timeValue(item.FinishTime), CreateBy: item.CreateBy,
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
		BuilderId: stringValue(item.BuilderId), BuilderAttempt: int32(item.BuilderAttempt),
		LeaseUntil: timeValue(item.LeaseUntil),
	}
}

func mapStorageObject(item *models.TStorageObject) *core.StorageObject {
	if item == nil {
		return nil
	}
	return &core.StorageObject{
		Id:           item.Id,
		TenantId:     item.TenantId,
		AppId:        item.AppId,
		ObjectType:   core.StorageObjectType(item.ObjectType),
		ObjectKey:    item.ObjectKey,
		OriginalName: item.OriginalName,
		ContentType:  item.ContentType,
		SizeBytes:    item.SizeBytes,
		Sha256:       stringValue(item.Sha256),
		Status:       core.StorageObjectStatus(item.Status),
		CreateBy:     item.CreateBy,
		CreateTime:   millis(item.CreateTime),
		UpdateTime:   millis(item.UpdateTime),
	}
}

func buildStatusToProto(value string) core.BuildTaskStatus {
	switch value {
	case buildStatusPending:
		return core.BuildTaskStatus_BUILD_TASK_STATUS_PENDING
	case buildStatusBuilding:
		return core.BuildTaskStatus_BUILD_TASK_STATUS_BUILDING
	case buildStatusSigning:
		return core.BuildTaskStatus_BUILD_TASK_STATUS_SIGNING
	case buildStatusUploading:
		return core.BuildTaskStatus_BUILD_TASK_STATUS_UPLOADING
	case buildStatusSuccess:
		return core.BuildTaskStatus_BUILD_TASK_STATUS_SUCCESS
	case buildStatusFailed:
		return core.BuildTaskStatus_BUILD_TASK_STATUS_FAILED
	case buildStatusCancelled:
		return core.BuildTaskStatus_BUILD_TASK_STATUS_CANCELLED
	default:
		return core.BuildTaskStatus_BUILD_TASK_STATUS_UNKNOWN
	}
}

func protoStatusToDB(value core.BuildTaskStatus) (string, error) {
	switch value {
	case core.BuildTaskStatus_BUILD_TASK_STATUS_PENDING:
		return buildStatusPending, nil
	case core.BuildTaskStatus_BUILD_TASK_STATUS_BUILDING:
		return buildStatusBuilding, nil
	case core.BuildTaskStatus_BUILD_TASK_STATUS_SIGNING:
		return buildStatusSigning, nil
	case core.BuildTaskStatus_BUILD_TASK_STATUS_UPLOADING:
		return buildStatusUploading, nil
	case core.BuildTaskStatus_BUILD_TASK_STATUS_SUCCESS:
		return buildStatusSuccess, nil
	case core.BuildTaskStatus_BUILD_TASK_STATUS_FAILED:
		return buildStatusFailed, nil
	case core.BuildTaskStatus_BUILD_TASK_STATUS_CANCELLED:
		return buildStatusCancelled, nil
	default:
		return "", status.Error(codes.InvalidArgument, "invalid build task status")
	}
}

func ensureTenant(itemTenant, requestedTenant int64) error {
	if itemTenant != requestedTenant {
		return status.Error(codes.NotFound, "resource not found")
	}
	return nil
}

func notFoundOrInternal(err error, resource string) error {
	if err == nil {
		return nil
	}
	if err == models.ErrNotFound || err == sqlx.ErrNotFound || err == sql.ErrNoRows {
		return status.Errorf(codes.NotFound, "%s not found", resource)
	}
	if grpcStatus, ok := status.FromError(err); ok && grpcStatus.Code() != codes.Unknown {
		return err
	}
	return status.Error(codes.Internal, fmt.Sprintf("%s query failed: %v", resource, err))
}

func pageValues(req *common.PageReq) (cursor, limit int64) {
	if req == nil {
		return 0, 20
	}
	cursor = req.Cursor
	limit = req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return cursor, limit
}

func pageBase(total, limit int64, items int, cursor int64, lastID int64) *common.RespBase {
	hasNext := int64(items) > limit
	if hasNext {
		items = int(limit)
	}
	if !hasNext {
		lastID = 0
	}
	return baseWithTotal(total, hasNext, lastID)
}
