package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"
)

func createPlatformBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformBrandingProfileReq) (*types.PlatformBrandingProfileResp, error) {
	item, err := svcCtx.CoreCli.CreateBrandingProfile(ctx, &core.CreateBrandingProfileReq{
		AppId: req.AppId, ProfileName: req.ProfileName, AppName: req.AppName,
		LogoObjectId: req.LogoObjectId, SplashObjectId: req.SplashObjectId, ApiHost: req.ApiHost,
		RewriteMode: core.BrandingRewriteMode(req.RewriteMode), LauncherIconTarget: req.LauncherIconTarget,
		SplashResourceTarget: req.SplashResourceTarget, RuntimeConfigJson: req.RuntimeConfigJson,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBrandingProfileResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBrandingProfile(item.Data)}, nil
}

func updatePlatformBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdatePlatformBrandingProfileReq) (*types.PlatformBrandingProfileResp, error) {
	item, err := svcCtx.CoreCli.UpdateBrandingProfile(ctx, &core.UpdateBrandingProfileReq{
		Id: req.Id, ProfileName: req.ProfileName, AppName: req.AppName,
		LogoObjectId: req.LogoObjectId, SplashObjectId: req.SplashObjectId, ApiHost: req.ApiHost,
		RewriteMode: core.BrandingRewriteMode(req.RewriteMode), LauncherIconTarget: req.LauncherIconTarget,
		SplashResourceTarget: req.SplashResourceTarget, RuntimeConfigJson: req.RuntimeConfigJson,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBrandingProfileResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBrandingProfile(item.Data)}, nil
}

func getPlatformBrandingProfile(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.PlatformBrandingProfileResp, error) {
	item, err := svcCtx.CoreCli.GetBrandingProfile(ctx, &core.BrandingProfileIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBrandingProfileResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBrandingProfile(item.Data)}, nil
}

func listPlatformBrandingProfiles(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformBrandingProfilesReq) (*types.PlatformBrandingProfileListResp, error) {
	item, err := svcCtx.CoreCli.ListBrandingProfiles(ctx, &core.BrandingProfileListReq{
		Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, Keyword: req.Keyword,
		Status: core.BrandingProfileStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformBrandingProfile, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformBrandingProfile(value))
	}
	return &types.PlatformBrandingProfileListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}

func changePlatformBrandingProfileStatus(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ChangePlatformBrandingProfileStatusReq) (*types.PlatformBrandingProfileResp, error) {
	item, err := svcCtx.CoreCli.ChangeBrandingProfileStatus(ctx, &core.ChangeBrandingProfileStatusReq{Id: req.Id, Status: core.BrandingProfileStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBrandingProfileResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBrandingProfile(item.Data)}, nil
}

func createPlatformBrandingPreflight(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformBrandingPreflightReq) (*types.PlatformBrandingPreflightResp, error) {
	item, err := svcCtx.CoreCli.CreateBrandingPreflight(ctx, &core.CreateBrandingPreflightReq{BrandingProfileId: req.Id, VersionId: req.VersionId})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBrandingPreflightResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBrandingPreflight(item.Data)}, nil
}

func getPlatformBrandingPreflight(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.PlatformBrandingPreflightResp, error) {
	item, err := svcCtx.CoreCli.GetBrandingPreflight(ctx, &core.BrandingPreflightIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformBrandingPreflightResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: platformlogic.MapPlatformBrandingPreflight(item.Data)}, nil
}

func listPlatformBrandingPreflights(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformBrandingPreflightsReq) (*types.PlatformBrandingPreflightListResp, error) {
	item, err := svcCtx.CoreCli.ListBrandingPreflights(ctx, &core.BrandingPreflightListReq{
		Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, BrandingProfileId: req.BrandingProfileId,
		VersionId: req.VersionId, Status: core.BrandingPreflightStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformBrandingPreflight, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, platformlogic.MapPlatformBrandingPreflight(value))
	}
	return &types.PlatformBrandingPreflightListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}
