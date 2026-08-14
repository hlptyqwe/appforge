package open_v1

import (
	"context"

	"appforge/admin-api/internal/logic/platform_private"
	"appforge/admin-api/internal/middleware"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func requireOpenScope(ctx context.Context, scope core.OpenApiScope, appID int64) error {
	if !middleware.RequireOpenApiScope(ctx, scope, appID) {
		return status.Errorf(codes.PermissionDenied, "%s scope or application access is required", scopeLabel(scope))
	}
	return nil
}

func requireListApp(ctx context.Context, scope core.OpenApiScope, appID int64) error {
	principal, ok := middleware.OpenApiPrincipalFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "API credential is required")
	}
	if len(principal.AppIDs) > 0 && appID <= 0 {
		return status.Error(codes.InvalidArgument, "appId is required for an application-scoped credential")
	}
	return requireOpenScope(ctx, scope, appID)
}

func scopeLabel(scope core.OpenApiScope) string {
	labels := map[core.OpenApiScope]string{
		core.OpenApiScope_OPEN_API_SCOPE_APPS_READ:      "apps:read",
		core.OpenApiScope_OPEN_API_SCOPE_APPS_WRITE:     "apps:write",
		core.OpenApiScope_OPEN_API_SCOPE_VERSIONS_READ:  "versions:read",
		core.OpenApiScope_OPEN_API_SCOPE_VERSIONS_WRITE: "versions:write",
		core.OpenApiScope_OPEN_API_SCOPE_CHANNELS_READ:  "channels:read",
		core.OpenApiScope_OPEN_API_SCOPE_CHANNELS_WRITE: "channels:write",
		core.OpenApiScope_OPEN_API_SCOPE_BRANDING_READ:  "branding:read",
		core.OpenApiScope_OPEN_API_SCOPE_BRANDING_WRITE: "branding:write",
		core.OpenApiScope_OPEN_API_SCOPE_BUILDS_READ:    "builds:read",
		core.OpenApiScope_OPEN_API_SCOPE_BUILDS_WRITE:   "builds:write",
		core.OpenApiScope_OPEN_API_SCOPE_ARTIFACTS_READ: "artifacts:read",
		core.OpenApiScope_OPEN_API_SCOPE_STATS_READ:     "stats:read",
	}
	return labels[scope]
}

func uploadScope(objectType int32) core.OpenApiScope {
	switch core.StorageObjectType(objectType) {
	case core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK:
		return core.OpenApiScope_OPEN_API_SCOPE_VERSIONS_WRITE
	default:
		return core.OpenApiScope_OPEN_API_SCOPE_BRANDING_WRITE
	}
}

func openV1CreateApplication(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformApplicationReq) (*types.PlatformApplicationResp, error) {
	principal, ok := middleware.OpenApiPrincipalFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "API credential is required")
	}
	if len(principal.AppIDs) > 0 {
		return nil, status.Error(codes.PermissionDenied, "application-scoped credentials cannot create applications")
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_APPS_WRITE, 0); err != nil {
		return nil, err
	}
	resp, err := platform_private.NewCreatePlatformApplicationLogic(ctx, svcCtx).CreatePlatformApplication(req)
	if err == nil {
		middleware.SetOpenApiResource(ctx, "application", resp.Data.Id)
	}
	return resp, err
}

func openV1GetApplication(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformApplicationResp, error) {
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_APPS_READ, req.Id); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "application", req.Id)
	return platform_private.NewGetPlatformApplicationLogic(ctx, svcCtx).GetPlatformApplication(req)
}

func openV1InitiateUpload(ctx context.Context, svcCtx *svc.ServiceContext, req *types.InitiatePlatformUploadReq) (*types.InitiatePlatformUploadResp, error) {
	if err := requireOpenScope(ctx, uploadScope(req.ObjectType), req.AppId); err != nil {
		return nil, err
	}
	resp, err := platform_private.NewInitiatePlatformUploadLogic(ctx, svcCtx).InitiatePlatformUpload(req)
	if err == nil {
		middleware.SetOpenApiResource(ctx, "storage_object", resp.Data.ObjectId)
	}
	return resp, err
}

func openV1CompleteUpload(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CompletePlatformUploadReq) (*types.CompletePlatformUploadResp, error) {
	object, err := svcCtx.CoreCli.GetStorageObject(ctx, &core.StorageObjectIdReq{Id: req.Id})
	if err != nil || object == nil || object.Data == nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, uploadScope(int32(object.Data.ObjectType)), object.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "storage_object", req.Id)
	return platform_private.NewCompletePlatformUploadLogic(ctx, svcCtx).CompletePlatformUpload(req)
}

func openV1ListVersions(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformVersionsReq) (*types.PlatformVersionListResp, error) {
	if err := requireListApp(ctx, core.OpenApiScope_OPEN_API_SCOPE_VERSIONS_READ, req.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "version", 0)
	return platform_private.NewListPlatformVersionsLogic(ctx, svcCtx).ListPlatformVersions(req)
}

func openV1CreateVersion(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformVersionReq) (*types.PlatformVersionResp, error) {
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_VERSIONS_WRITE, req.AppId); err != nil {
		return nil, err
	}
	resp, err := platform_private.NewCreatePlatformVersionLogic(ctx, svcCtx).CreatePlatformVersion(req)
	if err == nil {
		middleware.SetOpenApiResource(ctx, "version", resp.Data.Id)
	}
	return resp, err
}

func openV1GetVersion(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformVersionResp, error) {
	resp, err := platform_private.NewGetPlatformVersionLogic(ctx, svcCtx).GetPlatformVersion(req)
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_VERSIONS_READ, resp.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "version", req.Id)
	return resp, nil
}

func openV1ListChannels(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformChannelsReq) (*types.PlatformChannelListResp, error) {
	if err := requireListApp(ctx, core.OpenApiScope_OPEN_API_SCOPE_CHANNELS_READ, req.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "channel", 0)
	return platform_private.NewListPlatformChannelsLogic(ctx, svcCtx).ListPlatformChannels(req)
}

func openV1CreateChannel(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformChannelReq) (*types.PlatformChannelResp, error) {
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_CHANNELS_WRITE, req.AppId); err != nil {
		return nil, err
	}
	resp, err := platform_private.NewCreatePlatformChannelLogic(ctx, svcCtx).CreatePlatformChannel(req)
	if err == nil {
		middleware.SetOpenApiResource(ctx, "channel", resp.Data.Id)
	}
	return resp, err
}

func openV1GetChannel(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformChannelResp, error) {
	resp, err := platform_private.NewGetPlatformChannelLogic(ctx, svcCtx).GetPlatformChannel(req)
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_CHANNELS_READ, resp.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "channel", req.Id)
	return resp, nil
}

func openV1ListBrandingProfiles(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformBrandingProfilesReq) (*types.PlatformBrandingProfileListResp, error) {
	if err := requireListApp(ctx, core.OpenApiScope_OPEN_API_SCOPE_BRANDING_READ, req.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "branding_profile", 0)
	return platform_private.NewListPlatformBrandingProfilesLogic(ctx, svcCtx).ListPlatformBrandingProfiles(req)
}

func openV1CreateBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformBrandingProfileReq) (*types.PlatformBrandingProfileResp, error) {
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BRANDING_WRITE, req.AppId); err != nil {
		return nil, err
	}
	resp, err := platform_private.NewCreatePlatformBrandingProfileLogic(ctx, svcCtx).CreatePlatformBrandingProfile(req)
	if err == nil {
		middleware.SetOpenApiResource(ctx, "branding_profile", resp.Data.Id)
	}
	return resp, err
}

func openV1GetBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformBrandingProfileResp, error) {
	resp, err := platform_private.NewGetPlatformBrandingProfileLogic(ctx, svcCtx).GetPlatformBrandingProfile(req)
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BRANDING_READ, resp.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "branding_profile", req.Id)
	return resp, nil
}

func openV1UpdateBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdatePlatformBrandingProfileReq) (*types.PlatformBrandingProfileResp, error) {
	current, err := platform_private.NewGetPlatformBrandingProfileLogic(ctx, svcCtx).GetPlatformBrandingProfile(&types.PlatformIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BRANDING_WRITE, current.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "branding_profile", req.Id)
	return platform_private.NewUpdatePlatformBrandingProfileLogic(ctx, svcCtx).UpdatePlatformBrandingProfile(req)
}

func openV1ListWhiteLabelProducts(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformWhiteLabelProductsReq) (*types.PlatformWhiteLabelProductListResp, error) {
	if err := requireListApp(ctx, core.OpenApiScope_OPEN_API_SCOPE_BRANDING_READ, req.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "white_label_product", 0)
	return platform_private.NewListPlatformWhiteLabelProductsLogic(ctx, svcCtx).ListPlatformWhiteLabelProducts(req)
}

func openV1CreateWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformWhiteLabelProductReq) (*types.PlatformWhiteLabelProductResp, error) {
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BRANDING_WRITE, req.AppId); err != nil {
		return nil, err
	}
	resp, err := platform_private.NewCreatePlatformWhiteLabelProductLogic(ctx, svcCtx).CreatePlatformWhiteLabelProduct(req)
	if err == nil {
		middleware.SetOpenApiResource(ctx, "white_label_product", resp.Data.Id)
	}
	return resp, err
}

func openV1GetWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformWhiteLabelProductResp, error) {
	resp, err := platform_private.NewGetPlatformWhiteLabelProductLogic(ctx, svcCtx).GetPlatformWhiteLabelProduct(req)
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BRANDING_READ, resp.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "white_label_product", req.Id)
	return resp, nil
}

func openV1UpdateWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdatePlatformWhiteLabelProductReq) (*types.PlatformWhiteLabelProductResp, error) {
	current, err := platform_private.NewGetPlatformWhiteLabelProductLogic(ctx, svcCtx).GetPlatformWhiteLabelProduct(&types.PlatformIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BRANDING_WRITE, current.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "white_label_product", req.Id)
	return platform_private.NewUpdatePlatformWhiteLabelProductLogic(ctx, svcCtx).UpdatePlatformWhiteLabelProduct(req)
}

func openV1ListBuilds(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformBuildTasksReq) (*types.PlatformBuildTaskListResp, error) {
	if err := requireListApp(ctx, core.OpenApiScope_OPEN_API_SCOPE_BUILDS_READ, req.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "build", 0)
	return platform_private.NewListPlatformBuildTasksLogic(ctx, svcCtx).ListPlatformBuildTasks(req)
}

func openV1CreateBuild(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformBuildTaskReq) (*types.PlatformBuildTaskResp, error) {
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BUILDS_WRITE, req.AppId); err != nil {
		return nil, err
	}
	resp, err := platform_private.NewCreatePlatformBuildTaskLogic(ctx, svcCtx).CreatePlatformBuildTask(req)
	if err == nil {
		middleware.SetOpenApiResource(ctx, "build", resp.Data.Id)
	}
	return resp, err
}

func openV1GetBuild(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformBuildTaskResp, error) {
	resp, err := platform_private.NewGetPlatformBuildTaskLogic(ctx, svcCtx).GetPlatformBuildTask(req)
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BUILDS_READ, resp.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "build", req.Id)
	return resp, nil
}

func openV1CancelBuild(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CancelPlatformBuildTaskReq) (*types.PlatformBuildTaskResp, error) {
	current, err := platform_private.NewGetPlatformBuildTaskLogic(ctx, svcCtx).GetPlatformBuildTask(&types.PlatformIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BUILDS_WRITE, current.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "build", req.Id)
	return platform_private.NewCancelPlatformBuildTaskLogic(ctx, svcCtx).CancelPlatformBuildTask(req)
}

func openV1RetryBuild(ctx context.Context, svcCtx *svc.ServiceContext, req *types.RetryPlatformBuildTaskReq) (*types.PlatformBuildTaskResp, error) {
	current, err := platform_private.NewGetPlatformBuildTaskLogic(ctx, svcCtx).GetPlatformBuildTask(&types.PlatformIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_BUILDS_WRITE, current.Data.AppId); err != nil {
		return nil, err
	}
	resp, err := platform_private.NewRetryPlatformBuildTaskLogic(ctx, svcCtx).RetryPlatformBuildTask(req)
	if err == nil {
		middleware.SetOpenApiResource(ctx, "build", resp.Data.Id)
	}
	return resp, err
}

func openV1GetArtifactDownload(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformStorageObjectIdReq) (*types.PlatformStorageDownloadResp, error) {
	object, err := svcCtx.CoreCli.GetStorageObject(ctx, &core.StorageObjectIdReq{Id: req.Id})
	if err != nil || object == nil || object.Data == nil {
		return nil, err
	}
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_ARTIFACTS_READ, object.Data.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "artifact", req.Id)
	return platform_private.NewGetPlatformStorageDownloadLogic(ctx, svcCtx).GetPlatformStorageDownload(req)
}

func openV1GetChannelStats(ctx context.Context, svcCtx *svc.ServiceContext, req *types.GetPlatformChannelStatsReq) (*types.PlatformChannelStatsResp, error) {
	if err := requireOpenScope(ctx, core.OpenApiScope_OPEN_API_SCOPE_STATS_READ, req.AppId); err != nil {
		return nil, err
	}
	middleware.SetOpenApiResource(ctx, "channel_stats", req.ChannelId)
	return platform_private.NewGetPlatformChannelStatsLogic(ctx, svcCtx).GetPlatformChannelStats(req)
}
