package platform_private

import (
	"context"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/proto/common"
	"appforge/proto/core"
)

func createPlatformWebhookEndpoint(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformWebhookEndpointReq) (*types.PlatformWebhookEndpointSecretResp, error) {
	item, err := svcCtx.CoreCli.CreateWebhookEndpoint(ctx, &core.CreateWebhookEndpointReq{EndpointName: req.EndpointName,
		EndpointUrl: req.EndpointUrl, EventTypes: req.EventTypes, MaxAttempts: req.MaxAttempts})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWebhookEndpointSecretResp{RespBase: platformlogic.PlatformRespBase(item.Base),
		Data: mapPlatformWebhookEndpointSecret(item.Data)}, nil
}

func updatePlatformWebhookEndpoint(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdatePlatformWebhookEndpointReq) (*types.PlatformWebhookEndpointResp, error) {
	item, err := svcCtx.CoreCli.UpdateWebhookEndpoint(ctx, &core.UpdateWebhookEndpointReq{Id: req.Id,
		EndpointName: req.EndpointName, EndpointUrl: req.EndpointUrl, EventTypes: req.EventTypes,
		MaxAttempts: req.MaxAttempts, Status: core.WebhookEndpointStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWebhookEndpointResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: mapPlatformWebhookEndpoint(item.Data)}, nil
}

func getPlatformWebhookEndpoint(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformWebhookEndpointResp, error) {
	item, err := svcCtx.CoreCli.GetWebhookEndpoint(ctx, &core.WebhookEndpointIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWebhookEndpointResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: mapPlatformWebhookEndpoint(item.Data)}, nil
}

func listPlatformWebhookEndpoints(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformWebhookEndpointsReq) (*types.PlatformWebhookEndpointListResp, error) {
	item, err := svcCtx.CoreCli.ListWebhookEndpoints(ctx, &core.WebhookEndpointListReq{Page: platformlogic.PlatformPage(req.PageReq),
		Status: core.WebhookEndpointStatus(req.Status), Keyword: req.Keyword})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformWebhookEndpoint, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, mapPlatformWebhookEndpoint(value))
	}
	return &types.PlatformWebhookEndpointListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}

func rotatePlatformWebhookEndpointSecret(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformWebhookEndpointSecretResp, error) {
	item, err := svcCtx.CoreCli.RotateWebhookEndpointSecret(ctx, &core.WebhookEndpointIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWebhookEndpointSecretResp{RespBase: platformlogic.PlatformRespBase(item.Base),
		Data: mapPlatformWebhookEndpointSecret(item.Data)}, nil
}

func listPlatformWebhookDeliveries(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformWebhookDeliveriesReq) (*types.PlatformWebhookDeliveryListResp, error) {
	item, err := svcCtx.CoreCli.ListWebhookDeliveries(ctx, &core.WebhookDeliveryListReq{Page: platformlogic.PlatformPage(req.PageReq),
		EndpointId: req.EndpointId, Status: core.WebhookDeliveryStatus(req.Status), EventType: req.EventType})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformWebhookDelivery, 0, len(item.Data))
	for _, value := range item.Data {
		data = append(data, mapPlatformWebhookDelivery(value))
	}
	return &types.PlatformWebhookDeliveryListResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: data}, nil
}

func replayPlatformWebhookDelivery(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.PlatformWebhookDeliveryResp, error) {
	item, err := svcCtx.CoreCli.ReplayWebhookDelivery(ctx, &core.WebhookDeliveryIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWebhookDeliveryResp{RespBase: platformlogic.PlatformRespBase(item.Base), Data: mapPlatformWebhookDelivery(item.Data)}, nil
}

func testPlatformWebhookEndpoint(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformIdReq) (*types.RespBase, error) {
	item, err := svcCtx.CoreCli.CreateTestWebhookEvent(ctx, &core.CreateTestWebhookEventReq{EndpointId: req.Id})
	if err != nil {
		return nil, err
	}
	base := (*common.RespBase)(nil)
	if item != nil {
		base = item.Base
	}
	resp := platformlogic.PlatformRespBase(base)
	return &resp, nil
}

func mapPlatformWebhookEndpoint(item *core.WebhookEndpoint) types.PlatformWebhookEndpoint {
	if item == nil {
		return types.PlatformWebhookEndpoint{}
	}
	return types.PlatformWebhookEndpoint{Id: item.Id, TenantId: item.TenantId, EndpointName: item.EndpointName,
		EndpointUrl: item.EndpointUrl, EventTypes: item.EventTypes, SecretHint: item.SecretHint,
		MaxAttempts: item.MaxAttempts, Status: int32(item.Status), LastSuccessAt: item.LastSuccessAt,
		LastFailureAt: item.LastFailureAt, CreateBy: item.CreateBy, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}

func mapPlatformWebhookEndpointSecret(item *core.WebhookEndpointSecret) types.PlatformWebhookEndpointSecret {
	if item == nil {
		return types.PlatformWebhookEndpointSecret{}
	}
	return types.PlatformWebhookEndpointSecret{Endpoint: mapPlatformWebhookEndpoint(item.Endpoint), SigningSecret: item.SigningSecret}
}

func mapPlatformWebhookDelivery(item *core.WebhookDelivery) types.PlatformWebhookDelivery {
	if item == nil {
		return types.PlatformWebhookDelivery{}
	}
	return types.PlatformWebhookDelivery{Id: item.Id, TenantId: item.TenantId, EndpointId: item.EndpointId,
		EventId: item.EventId, EventType: item.EventType, Attempt: item.Attempt, Status: int32(item.Status),
		ResponseStatus: item.ResponseStatus, ResponseBodyExcerpt: item.ResponseBodyExcerpt, ErrorMessage: item.ErrorMessage,
		NextRetryAt: item.NextRetryAt, DeliveredAt: item.DeliveredAt, CreateTime: item.CreateTime, UpdateTime: item.UpdateTime}
}
