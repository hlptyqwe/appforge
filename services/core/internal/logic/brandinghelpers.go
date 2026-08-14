package logic

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"regexp"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var brandingResourceTargetPattern = regexp.MustCompile(`^[a-z0-9_]+/[a-z0-9_.]+$`)

type brandingSnapshot struct {
	ProfileID            int64  `json:"profileId"`
	Revision             int64  `json:"revision"`
	AppName              string `json:"appName"`
	LogoObjectID         int64  `json:"logoObjectId"`
	SplashObjectID       int64  `json:"splashObjectId"`
	APIHost              string `json:"apiHost"`
	RewriteMode          int64  `json:"rewriteMode"`
	LauncherIconTarget   string `json:"launcherIconTarget,omitempty"`
	SplashResourceTarget string `json:"splashResourceTarget,omitempty"`
	RuntimeConfigJSON    string `json:"runtimeConfig,omitempty"`
}

func validateBrandingFields(profileName, appName, apiHost string, rewriteMode core.BrandingRewriteMode, launcherTarget, splashTarget, runtimeConfig string) error {
	if err := requireText(profileName, "profile_name", 128); err != nil {
		return err
	}
	if err := requireText(appName, "app_name", 128); err != nil {
		return err
	}
	if err := validateBrandingAPIHost(apiHost); err != nil {
		return err
	}
	if rewriteMode != core.BrandingRewriteMode_BRANDING_REWRITE_MODE_RESOURCE_REBUILD &&
		rewriteMode != core.BrandingRewriteMode_BRANDING_REWRITE_MODE_RUNTIME_CONTRACT {
		return status.Error(codes.InvalidArgument, "invalid rewrite_mode")
	}
	if err := requireOptionalText(launcherTarget, "launcher_icon_target", 255); err != nil {
		return err
	}
	if !brandingResourceTargetPattern.MatchString(strings.TrimPrefix(strings.TrimSpace(launcherTarget), "@")) {
		return status.Error(codes.InvalidArgument, "launcher_icon_target must use type/resource_name format")
	}
	if err := requireOptionalText(splashTarget, "splash_resource_target", 255); err != nil {
		return err
	}
	if rewriteMode == core.BrandingRewriteMode_BRANDING_REWRITE_MODE_RESOURCE_REBUILD &&
		!brandingResourceTargetPattern.MatchString(strings.TrimPrefix(strings.TrimSpace(splashTarget), "@")) {
		return status.Error(codes.InvalidArgument, "splash_resource_target must use type/resource_name format")
	}
	if config := strings.TrimSpace(runtimeConfig); config != "" && !json.Valid([]byte(config)) {
		return status.Error(codes.InvalidArgument, "runtime_config_json must be valid JSON")
	}
	return nil
}

func validateBrandingAPIHost(value string) error {
	raw := strings.TrimSpace(value)
	if len(raw) > 500 {
		return status.Error(codes.InvalidArgument, "api_host is too long")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return status.Error(codes.InvalidArgument, "api_host must be an absolute origin URL")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return status.Error(codes.InvalidArgument, "api_host must not contain a path")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme == "http" && (host == "localhost" || host == "::1" || net.ParseIP(host).IsLoopback()) {
		return nil
	}
	return status.Error(codes.InvalidArgument, "api_host must use HTTPS; HTTP is only allowed for localhost")
}

func validateBrandingObject(ctx context.Context, svcCtx *svc.ServiceContext, tenantID, appID, objectID int64, objectType core.StorageObjectType) (*models.TStorageObject, error) {
	if err := requirePositive(objectID, "branding object id"); err != nil {
		return nil, err
	}
	item, err := svcCtx.StorageObjectModel.FindOne(ctx, objectID)
	if err != nil {
		return nil, notFoundOrInternal(err, "branding object")
	}
	if item.TenantId != tenantID || item.AppId != appID || item.ObjectType != int64(objectType) ||
		(item.Status != storageStatusReady && item.Status != storageStatusBound) {
		return nil, status.Error(codes.InvalidArgument, "branding object is invalid or not ready")
	}
	return item, nil
}

func prepareBrandingSnapshot(ctx context.Context, svcCtx *svc.ServiceContext, tenantID, appID, versionID, profileID int64) (int64, string, error) {
	if profileID <= 0 {
		return 0, "", nil
	}
	profile, err := svcCtx.BrandingProfileModel.FindOne(ctx, profileID)
	if err != nil {
		return 0, "", notFoundOrInternal(err, "branding profile")
	}
	if profile.TenantId != tenantID || profile.AppId != appID || profile.Status != int64(core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_ENABLED) {
		return 0, "", status.Error(codes.InvalidArgument, "branding profile is invalid or not enabled")
	}
	if _, err := validateBrandingObject(ctx, svcCtx, tenantID, appID, profile.LogoObjectId, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_LOGO); err != nil {
		return 0, "", err
	}
	if _, err := validateBrandingObject(ctx, svcCtx, tenantID, appID, profile.SplashObjectId, core.StorageObjectType_STORAGE_OBJECT_TYPE_BRAND_SPLASH); err != nil {
		return 0, "", err
	}
	preflight, err := svcCtx.BrandingPreflightModel.FindOneByTenantIdBrandingProfileIdBrandingRevisionVersionId(ctx, tenantID, profile.Id, profile.Revision, versionID)
	if err != nil || preflight.Status != int64(core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_COMPATIBLE) {
		return 0, "", status.Error(codes.FailedPrecondition, "a compatible preflight for this branding revision and version is required")
	}
	snapshot := brandingSnapshot{
		ProfileID: profile.Id, Revision: profile.Revision, AppName: profile.AppName,
		LogoObjectID: profile.LogoObjectId, SplashObjectID: profile.SplashObjectId,
		APIHost: profile.ApiHost, RewriteMode: profile.RewriteMode,
		LauncherIconTarget:   stringValue(profile.LauncherIconTarget),
		SplashResourceTarget: stringValue(profile.SplashResourceTarget),
		RuntimeConfigJSON:    stringValue(profile.RuntimeConfig),
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return 0, "", status.Error(codes.Internal, "encode branding snapshot failed")
	}
	return profile.Revision, string(encoded), nil
}

func parseBrandingSnapshot(value string) (*brandingSnapshot, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var snapshot brandingSnapshot
	if err := json.Unmarshal([]byte(value), &snapshot); err != nil || snapshot.ProfileID <= 0 || snapshot.Revision <= 0 {
		return nil, status.Error(codes.FailedPrecondition, "branding snapshot is invalid")
	}
	return &snapshot, nil
}
