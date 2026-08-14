package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const brandingProfileSelect = `SELECT id, tenant_id, app_id, profile_name, app_name, logo_object_id,
splash_object_id, api_host, rewrite_mode, launcher_icon_target, splash_resource_target, runtime_config,
status, revision, create_by, create_time, update_time FROM t_app_branding_profile`

const brandingPreflightSelect = `SELECT id, tenant_id, app_id, branding_profile_id, branding_revision,
version_id, status, report_json, source_apk_sha256, toolchain_version, builder_id, builder_attempt,
start_time, finish_time, lease_until,
create_by, create_time, update_time FROM t_branding_preflight`

func createBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateBrandingProfileReq) (*core.BrandingProfileResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.AppId, "app_id"); err != nil {
		return nil, err
	}
	if err := validateBrandingFields(in.ProfileName, in.AppName, in.ApiHost, in.RewriteMode, in.LauncherIconTarget, in.SplashResourceTarget, in.RuntimeConfigJson); err != nil {
		return nil, err
	}
	app, err := svcCtx.ApplicationModel.FindOne(ctx, in.AppId)
	if err != nil {
		return nil, notFoundOrInternal(err, "application")
	}
	if err := ensureTenant(app.TenantId, tenant); err != nil {
		return nil, err
	}
	logo, err := validateBrandingObject(ctx, svcCtx, tenant, in.AppId, in.LogoObjectId, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO)
	if err != nil {
		return nil, err
	}
	splash, err := validateBrandingObject(ctx, svcCtx, tenant, in.AppId, in.SplashObjectId, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH)
	if err != nil {
		return nil, err
	}
	profileName := strings.TrimSpace(in.ProfileName)
	if existing, findErr := svcCtx.BrandingProfileModel.FindOneByTenantIdAppIdProfileName(ctx, tenant, in.AppId, profileName); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "branding profile name already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Error(codes.Internal, "check branding profile name failed")
	}
	result, err := svcCtx.BrandingProfileModel.Insert(ctx, &models.TAppBrandingProfile{
		TenantId: tenant, AppId: in.AppId, ProfileName: profileName, AppName: strings.TrimSpace(in.AppName),
		LogoObjectId: in.LogoObjectId, SplashObjectId: in.SplashObjectId, ApiHost: strings.TrimSpace(in.ApiHost),
		RewriteMode: int64(in.RewriteMode), LauncherIconTarget: nullString(in.LauncherIconTarget),
		SplashResourceTarget: nullString(in.SplashResourceTarget), RuntimeConfig: nullString(in.RuntimeConfigJson),
		Status: int64(core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_DRAFT), Revision: 1, CreateBy: actorID(ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create branding profile failed: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read branding profile id failed: %v", err)
	}
	if err := bindBrandingObjects(ctx, svcCtx, logo, splash); err != nil {
		return nil, err
	}
	item, err := svcCtx.BrandingProfileModel.FindOne(ctx, id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding profile")
	}
	return &core.BrandingProfileResp{Base: okBase(), Data: mapBrandingProfile(item)}, nil
}

func updateBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, in *core.UpdateBrandingProfileReq) (*core.BrandingProfileResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.Id, "id"); err != nil {
		return nil, err
	}
	if err := validateBrandingFields(in.ProfileName, in.AppName, in.ApiHost, in.RewriteMode, in.LauncherIconTarget, in.SplashResourceTarget, in.RuntimeConfigJson); err != nil {
		return nil, err
	}
	item, err := svcCtx.BrandingProfileModel.FindOne(ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding profile")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}
	logo, err := validateBrandingObject(ctx, svcCtx, tenant, item.AppId, in.LogoObjectId, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO)
	if err != nil {
		return nil, err
	}
	splash, err := validateBrandingObject(ctx, svcCtx, tenant, item.AppId, in.SplashObjectId, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH)
	if err != nil {
		return nil, err
	}
	profileName := strings.TrimSpace(in.ProfileName)
	var existingID int64
	findErr := svcCtx.DB.QueryRowCtx(ctx, &existingID, `SELECT id FROM t_app_branding_profile WHERE tenant_id = ? AND app_id = ? AND profile_name = ? LIMIT 1`, tenant, item.AppId, profileName)
	if findErr == nil && existingID != item.Id {
		return nil, status.Error(codes.AlreadyExists, "branding profile name already exists")
	} else if findErr != nil && findErr != sql.ErrNoRows && findErr != sqlx.ErrNotFound {
		return nil, status.Error(codes.Internal, "check branding profile name failed")
	}
	item.ProfileName = profileName
	item.AppName = strings.TrimSpace(in.AppName)
	item.LogoObjectId = in.LogoObjectId
	item.SplashObjectId = in.SplashObjectId
	item.ApiHost = strings.TrimSpace(in.ApiHost)
	item.RewriteMode = int64(in.RewriteMode)
	item.LauncherIconTarget = nullString(in.LauncherIconTarget)
	item.SplashResourceTarget = nullString(in.SplashResourceTarget)
	item.RuntimeConfig = nullString(in.RuntimeConfigJson)
	item.Revision++
	if err := svcCtx.BrandingProfileModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update branding profile failed: %v", err)
	}
	if err := bindBrandingObjects(ctx, svcCtx, logo, splash); err != nil {
		return nil, err
	}
	item, err = svcCtx.BrandingProfileModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding profile")
	}
	return &core.BrandingProfileResp{Base: okBase(), Data: mapBrandingProfile(item)}, nil
}

func getBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BrandingProfileIdReq) (*core.BrandingProfileResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.Id, "id"); err != nil {
		return nil, err
	}
	item, err := svcCtx.BrandingProfileModel.FindOne(ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding profile")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}
	return &core.BrandingProfileResp{Base: okBase(), Data: mapBrandingProfile(item)}, nil
}

func listBrandingProfiles(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BrandingProfileListReq) (*core.BrandingProfileListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &core.BrandingProfileListReq{}
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	if in.AppId > 0 {
		where = append(where, "app_id = ?")
		args = append(args, in.AppId)
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		where = append(where, "(profile_name LIKE ? OR app_name LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	if in.Status != core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(in.Status))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM t_app_branding_profile WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list branding profiles count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TAppBrandingProfile
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, fmt.Sprintf("%s WHERE %s AND id > ? ORDER BY id ASC LIMIT ?", brandingProfileSelect, whereSQL), queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list branding profiles failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.BrandingProfile, 0, len(items))
	var nextCursor int64
	for i := range items {
		data = append(data, mapBrandingProfile(&items[i]))
		nextCursor = items[i].Id
	}
	if !hasNext {
		nextCursor = 0
	}
	return &core.BrandingProfileListResp{Base: baseWithTotal(total, hasNext, nextCursor), Data: data}, nil
}

func changeBrandingProfileStatus(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ChangeBrandingProfileStatusReq) (*core.BrandingProfileResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.Status != core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_DRAFT &&
		in.Status != core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_ENABLED &&
		in.Status != core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_DISABLED {
		return nil, status.Error(codes.InvalidArgument, "invalid branding profile status")
	}
	item, err := svcCtx.BrandingProfileModel.FindOne(ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding profile")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}
	item.Status = int64(in.Status)
	if err := svcCtx.BrandingProfileModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "change branding profile status failed: %v", err)
	}
	item, err = svcCtx.BrandingProfileModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding profile")
	}
	return &core.BrandingProfileResp{Base: okBase(), Data: mapBrandingProfile(item)}, nil
}

func createBrandingPreflight(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateBrandingPreflightReq) (*core.BrandingPreflightResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.BrandingProfileId, "branding_profile_id"); err != nil {
		return nil, err
	}
	if err := requirePositive(in.VersionId, "version_id"); err != nil {
		return nil, err
	}
	profile, err := svcCtx.BrandingProfileModel.FindOne(ctx, in.BrandingProfileId)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding profile")
	}
	if err := ensureTenant(profile.TenantId, tenant); err != nil {
		return nil, err
	}
	version, err := svcCtx.VersionModel.FindOne(ctx, in.VersionId)
	if err != nil {
		return nil, notFoundOrInternal(err, "version")
	}
	if version.TenantId != tenant || version.AppId != profile.AppId {
		return nil, status.Error(codes.NotFound, "version not found")
	}
	if version.SourceApkObjectId <= 0 || strings.TrimSpace(stringValue(version.SourceApkSha256)) == "" {
		return nil, status.Error(codes.FailedPrecondition, "version has no validated source APK")
	}
	now := time.Now()
	item, findErr := svcCtx.BrandingPreflightModel.FindOneByTenantIdBrandingProfileIdBrandingRevisionVersionId(ctx, tenant, profile.Id, profile.Revision, version.Id)
	if findErr == nil {
		item.Status = int64(core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_PENDING)
		item.ReportJson = nullString("")
		item.SourceApkSha256 = version.SourceApkSha256
		item.ToolchainVersion = nullString("")
		item.BuilderId = nullString("")
		item.StartTime = nullTime(now)
		item.FinishTime = nullTime(time.Time{})
		item.LeaseUntil = nullTime(time.Time{})
		item.CreateBy = actorID(ctx)
		if err := svcCtx.BrandingPreflightModel.Update(ctx, item); err != nil {
			return nil, status.Errorf(codes.Internal, "reset branding preflight failed: %v", err)
		}
	} else if findErr == models.ErrNotFound {
		result, insertErr := svcCtx.BrandingPreflightModel.Insert(ctx, &models.TBrandingPreflight{
			TenantId: tenant, AppId: profile.AppId, BrandingProfileId: profile.Id, BrandingRevision: profile.Revision,
			VersionId: version.Id, Status: int64(core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_PENDING),
			SourceApkSha256: version.SourceApkSha256, StartTime: nullTime(now), CreateBy: actorID(ctx),
		})
		if insertErr != nil {
			return nil, status.Errorf(codes.Internal, "create branding preflight failed: %v", insertErr)
		}
		id, idErr := result.LastInsertId()
		if idErr != nil {
			return nil, status.Errorf(codes.Internal, "read branding preflight id failed: %v", idErr)
		}
		item, err = svcCtx.BrandingPreflightModel.FindOne(ctx, id)
		if err != nil {
			return nil, notFoundOrInternal(err, "branding preflight")
		}
	} else {
		return nil, status.Errorf(codes.Internal, "check branding preflight failed: %v", findErr)
	}
	item, err = svcCtx.BrandingPreflightModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding preflight")
	}
	return &core.BrandingPreflightResp{Base: okBase(), Data: mapBrandingPreflight(item)}, nil
}

func getBrandingPreflight(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BrandingPreflightIdReq) (*core.BrandingPreflightResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	item, err := svcCtx.BrandingPreflightModel.FindOne(ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding preflight")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}
	return &core.BrandingPreflightResp{Base: okBase(), Data: mapBrandingPreflight(item)}, nil
}

func listBrandingPreflights(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BrandingPreflightListReq) (*core.BrandingPreflightListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &core.BrandingPreflightListReq{}
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	for _, filter := range []struct {
		column string
		value  int64
	}{{"app_id", in.AppId}, {"branding_profile_id", in.BrandingProfileId}, {"version_id", in.VersionId}} {
		if filter.value > 0 {
			where = append(where, filter.column+" = ?")
			args = append(args, filter.value)
		}
	}
	if in.Status != core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(in.Status))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, fmt.Sprintf("SELECT COUNT(1) FROM t_branding_preflight WHERE %s", whereSQL), args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list branding preflights count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TBrandingPreflight
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, fmt.Sprintf("%s WHERE %s AND id > ? ORDER BY id ASC LIMIT ?", brandingPreflightSelect, whereSQL), queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list branding preflights failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.BrandingPreflight, 0, len(items))
	var nextCursor int64
	for i := range items {
		data = append(data, mapBrandingPreflight(&items[i]))
		nextCursor = items[i].Id
	}
	if !hasNext {
		nextCursor = 0
	}
	return &core.BrandingPreflightListResp{Base: baseWithTotal(total, hasNext, nextCursor), Data: data}, nil
}

func completeBrandingPreflight(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CompleteBrandingPreflightReq) (*core.BrandingPreflightResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := requirePositive(in.Id, "id"); err != nil {
		return nil, err
	}
	if in.Status != core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_COMPATIBLE &&
		in.Status != core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_INCOMPATIBLE &&
		in.Status != core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_FAILED {
		return nil, status.Error(codes.InvalidArgument, "invalid completed preflight status")
	}
	if !json.Valid([]byte(strings.TrimSpace(in.ReportJson))) {
		return nil, status.Error(codes.InvalidArgument, "report_json must be valid JSON")
	}
	if err := requireText(in.ToolchainVersion, "toolchain_version", 128); err != nil {
		return nil, err
	}
	if err := validateBuilderRequest(in.BuilderId); err != nil {
		return nil, err
	}
	if err := requirePositive(int64(in.BuilderAttempt), "builder_attempt"); err != nil {
		return nil, err
	}
	item, err := svcCtx.BrandingPreflightModel.FindOne(ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding preflight")
	}
	if item.Status != int64(core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_PENDING) ||
		stringValue(item.BuilderId) != strings.TrimSpace(in.BuilderId) || item.BuilderAttempt != int64(in.BuilderAttempt) ||
		!item.LeaseUntil.Valid || item.LeaseUntil.Time.Before(time.Now()) {
		return nil, status.Error(codes.FailedPrecondition, "branding preflight is not owned by builder or lease has expired")
	}
	item.Status = int64(in.Status)
	item.ReportJson = nullString(in.ReportJson)
	item.ToolchainVersion = nullString(in.ToolchainVersion)
	item.FinishTime = nullTime(time.Now())
	if err := svcCtx.BrandingPreflightModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "complete branding preflight failed: %v", err)
	}
	item, err = svcCtx.BrandingPreflightModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding preflight")
	}
	return &core.BrandingPreflightResp{Base: okBase(), Data: mapBrandingPreflight(item)}, nil
}

func claimBrandingPreflight(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ClaimBrandingPreflightReq) (*core.BrandingPreflightExecutionContextResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateBuilderRequest(in.BuilderId); err != nil {
		return nil, err
	}
	seconds := leaseSeconds(in.LeaseSeconds)
	var item models.TBrandingPreflight
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		query := brandingPreflightSelect + ` WHERE status = ? AND (builder_id IS NULL OR lease_until IS NULL OR lease_until < CURRENT_TIMESTAMP)
AND EXISTS (SELECT 1 FROM t_app_branding_profile p WHERE p.id = t_branding_preflight.branding_profile_id AND p.revision = t_branding_preflight.branding_revision)
ORDER BY id ASC LIMIT 1 FOR UPDATE`
		if err := session.QueryRowCtx(txCtx, &item, query, int64(core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_PENDING)); err != nil {
			return err
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_branding_preflight SET builder_id = ?, builder_attempt = builder_attempt + 1,
start_time = CURRENT_TIMESTAMP, lease_until = DATE_ADD(CURRENT_TIMESTAMP, INTERVAL ? SECOND), update_time = CURRENT_TIMESTAMP
WHERE id = ? AND status = ?`, strings.TrimSpace(in.BuilderId), seconds, item.Id, int64(core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_PENDING))
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return sql.ErrNoRows
		}
		item.BuilderId = nullString(in.BuilderId)
		item.BuilderAttempt++
		item.StartTime = nullTime(time.Now())
		item.LeaseUntil = nullTime(time.Now().Add(time.Duration(seconds) * time.Second))
		return nil
	})
	if err != nil {
		if err == sql.ErrNoRows || err == sqlx.ErrNotFound {
			return &core.BrandingPreflightExecutionContextResp{Base: okBase()}, nil
		}
		return nil, status.Errorf(codes.Internal, "claim branding preflight failed: %v", err)
	}
	// 原子领取使用事务直写；再通过缓存模型写入同一快照以清除该记录的旧缓存。
	if err := svcCtx.BrandingPreflightModel.Update(ctx, &item); err != nil {
		return nil, status.Errorf(codes.Internal, "refresh claimed branding preflight cache failed: %v", err)
	}
	profile, err := svcCtx.BrandingProfileModel.FindOne(ctx, item.BrandingProfileId)
	if err != nil || profile.TenantId != item.TenantId || profile.Revision != item.BrandingRevision {
		return nil, status.Error(codes.FailedPrecondition, "branding profile revision is unavailable")
	}
	version, err := svcCtx.VersionModel.FindOne(ctx, item.VersionId)
	if err != nil || version.TenantId != item.TenantId || version.AppId != item.AppId {
		return nil, status.Error(codes.FailedPrecondition, "branding preflight version is unavailable")
	}
	source, err := svcCtx.StorageObjectModel.FindOne(ctx, version.SourceApkObjectId)
	if err != nil || source.TenantId != item.TenantId || source.Status != storageStatusBound {
		return nil, status.Error(codes.FailedPrecondition, "branding preflight source APK is unavailable")
	}
	logo, err := validateBrandingObject(ctx, svcCtx, item.TenantId, item.AppId, profile.LogoObjectId, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO)
	if err != nil {
		return nil, err
	}
	splash, err := validateBrandingObject(ctx, svcCtx, item.TenantId, item.AppId, profile.SplashObjectId, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH)
	if err != nil {
		return nil, err
	}
	return &core.BrandingPreflightExecutionContextResp{Base: okBase(), Data: &core.BrandingPreflightExecutionContext{
		Preflight: mapBrandingPreflight(&item), Profile: mapBrandingProfile(profile), Version: mapVersion(version),
		SourceApk: mapStorageObject(source), BrandLogo: mapStorageObject(logo), BrandSplash: mapStorageObject(splash),
	}}, nil
}

func bindBrandingObjects(ctx context.Context, svcCtx *svc.ServiceContext, items ...*models.TStorageObject) error {
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item == nil || item.Status == storageStatusBound {
			continue
		}
		if _, ok := seen[item.Id]; ok {
			continue
		}
		seen[item.Id] = struct{}{}
		item.Status = storageStatusBound
		if err := svcCtx.StorageObjectModel.Update(ctx, item); err != nil {
			return status.Errorf(codes.Internal, "bind branding object failed: %v", err)
		}
	}
	return nil
}
