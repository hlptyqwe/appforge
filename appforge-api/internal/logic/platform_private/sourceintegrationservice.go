package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/core"
)

func listPlatformSourceIntegrations(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformSourceIntegrationsReq) (*types.PlatformSourceIntegrationListResp, error) {
	item, err := svcCtx.CoreCli.ListSourceIntegrations(ctx, &core.SourceIntegrationListReq{Page: platformlogic.PlatformPage(req.PageReq),
		Platform: core.SourcePlatform(req.Platform), Status: core.SourceIntegrationStatus(req.Status), Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformSourceIntegration, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, mapPlatformSourceIntegration(value))
	}
	return &types.PlatformSourceIntegrationListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}

func getPlatformSourceIntegration(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformSourceIntegrationResp, error) {
	item, err := svcCtx.CoreCli.GetSourceIntegration(ctx, &core.SourceIntegrationIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceIntegrationResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: mapPlatformSourceIntegration(item.Data)}, nil
}

func disconnectPlatformSourceIntegration(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformSourceIntegrationResp, error) {
	item, err := svcCtx.CoreCli.DisconnectSourceIntegration(ctx, &core.SourceIntegrationIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceIntegrationResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: mapPlatformSourceIntegration(item.Data)}, nil
}

func listPlatformSourceRepositories(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformSourceRepositoriesReq) (*types.PlatformSourceRepositoryListResp, error) {
	item, err := svcCtx.CoreCli.ListSourceRepositories(ctx, &core.SourceRepositoryListReq{Page: platformlogic.PlatformPage(req.PageReq),
		IntegrationId: req.IntegrationId, Status: core.SourceRepositoryStatus(req.Status), Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformSourceRepository, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, mapPlatformSourceRepository(value))
	}
	return &types.PlatformSourceRepositoryListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}

func revokePlatformSourceRepository(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformSourceRepositoryResp, error) {
	item, err := svcCtx.CoreCli.RevokeSourceRepository(ctx, &core.SourceRepositoryIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformSourceRepositoryResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: mapPlatformSourceRepository(item.Data)}, nil
}

func mapPlatformSourceIntegration(item *core.SourceIntegration) types.PlatformSourceIntegration {
	if item == nil {
		return types.PlatformSourceIntegration{}
	}
	return types.PlatformSourceIntegration{Id: item.Id, TenantId: item.TenantId, Platform: int32(item.Platform),
		IntegrationName: item.IntegrationName, InstallationRef: item.InstallationRef, TokenExpiresAt: item.TokenExpiresAt,
		Status: int32(item.Status), LastSyncAt: item.LastSyncAt, CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func mapPlatformSourceRepository(item *core.SourceRepository) types.PlatformSourceRepository {
	if item == nil {
		return types.PlatformSourceRepository{}
	}
	return types.PlatformSourceRepository{Id: item.Id, TenantId: item.TenantId, IntegrationId: item.IntegrationId,
		ExternalRepositoryId: item.ExternalRepositoryId, RepositoryFullName: item.RepositoryFullName, DefaultBranch: item.DefaultBranch,
		PermissionLevel: item.PermissionLevel, Status: int32(item.Status), CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}
