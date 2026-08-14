package platform_private

import (
	"context"
	"encoding/json"
	"strings"

	platformlogic "appforge/admin-api/internal/logic"
	"appforge/admin-api/internal/svc"
	"appforge/admin-api/internal/types"
	"appforge/common/secretbox"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func createPlatformWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformWhiteLabelTemplateReq) (*types.PlatformWhiteLabelTemplateResp, error) {
	response, err := svcCtx.CoreCli.CreateWhiteLabelTemplate(ctx, &core.CreateWhiteLabelTemplateReq{
		AppId: req.AppId, TemplateCode: req.TemplateCode, TemplateName: req.TemplateName,
		SourceVersionId: req.SourceVersionId, ParameterSchemaJson: req.ParameterSchemaJson,
		CompatibilityRulesJson: req.CompatibilityRulesJson,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplate(response.Data)}, nil
}

func updatePlatformWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdatePlatformWhiteLabelTemplateReq) (*types.PlatformWhiteLabelTemplateResp, error) {
	response, err := svcCtx.CoreCli.UpdateWhiteLabelTemplate(ctx, &core.UpdateWhiteLabelTemplateReq{
		Id: req.Id, TemplateName: req.TemplateName, SourceVersionId: req.SourceVersionId,
		ParameterSchemaJson: req.ParameterSchemaJson, CompatibilityRulesJson: req.CompatibilityRulesJson,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplate(response.Data)}, nil
}

func copyPlatformWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CopyPlatformWhiteLabelTemplateReq) (*types.PlatformWhiteLabelTemplateResp, error) {
	response, err := svcCtx.CoreCli.CopyWhiteLabelTemplate(ctx, &core.CopyWhiteLabelTemplateReq{
		Id: req.Id, TemplateCode: req.TemplateCode, TemplateName: req.TemplateName, SourceVersionId: req.SourceVersionId,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplate(response.Data)}, nil
}

func deletePlatformWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.RespBase, error) {
	response, err := svcCtx.CoreCli.DeleteWhiteLabelTemplate(ctx, &core.WhiteLabelTemplateIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	result := platformlogic.PlatformRespBase(response.Base)
	return &result, nil
}

func getPlatformWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.PlatformWhiteLabelTemplateResp, error) {
	response, err := svcCtx.CoreCli.GetWhiteLabelTemplate(ctx, &core.WhiteLabelTemplateIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplate(response.Data)}, nil
}

func listPlatformWhiteLabelTemplates(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformWhiteLabelTemplatesReq) (*types.PlatformWhiteLabelTemplateListResp, error) {
	response, err := svcCtx.CoreCli.ListWhiteLabelTemplates(ctx, &core.WhiteLabelTemplateListReq{
		Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, Keyword: req.Keyword,
		Status: core.WhiteLabelTemplateStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformWhiteLabelTemplate, 0, len(response.Data))
	for _, item := range response.Data {
		data = append(data, platformlogic.MapPlatformWhiteLabelTemplate(item))
	}
	return &types.PlatformWhiteLabelTemplateListResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: data}, nil
}

func createPlatformWhiteLabelTemplateRevision(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformWhiteLabelTemplateRevisionReq) (*types.PlatformWhiteLabelTemplateRevisionResp, error) {
	response, err := svcCtx.CoreCli.CreateWhiteLabelTemplateRevision(ctx, &core.CreateWhiteLabelTemplateRevisionReq{
		TemplateId: req.Id, PackageNameRuleJson: req.PackageNameRuleJson, ManifestPatchJson: req.ManifestPatchJson,
		ResourcePatchJson: req.ResourcePatchJson, ExtensionFilesJson: req.ExtensionFilesJson,
		ExpectedArtifactsJson: req.ExpectedArtifactsJson,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateRevisionResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplateRevision(response.Data)}, nil
}

func getPlatformWhiteLabelTemplateRevision(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformWhiteLabelTemplateRevisionIdReq) (*types.PlatformWhiteLabelTemplateRevisionResp, error) {
	response, err := svcCtx.CoreCli.GetWhiteLabelTemplateRevision(ctx, &core.WhiteLabelTemplateRevisionIdReq{TemplateId: req.Id, Revision: req.Revision})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateRevisionResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplateRevision(response.Data)}, nil
}

func updatePlatformWhiteLabelTemplateRevision(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdatePlatformWhiteLabelTemplateRevisionReq) (*types.PlatformWhiteLabelTemplateRevisionResp, error) {
	response, err := svcCtx.CoreCli.UpdateWhiteLabelTemplateRevision(ctx, &core.UpdateWhiteLabelTemplateRevisionReq{
		TemplateId: req.Id, Revision: req.Revision, PackageNameRuleJson: req.PackageNameRuleJson,
		ManifestPatchJson: req.ManifestPatchJson, ResourcePatchJson: req.ResourcePatchJson,
		ExtensionFilesJson: req.ExtensionFilesJson, ExpectedArtifactsJson: req.ExpectedArtifactsJson,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateRevisionResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplateRevision(response.Data)}, nil
}

func deletePlatformWhiteLabelTemplateRevision(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PlatformWhiteLabelTemplateRevisionIdReq) (*types.RespBase, error) {
	response, err := svcCtx.CoreCli.DeleteWhiteLabelTemplateRevision(ctx, &core.WhiteLabelTemplateRevisionIdReq{TemplateId: req.Id, Revision: req.Revision})
	if err != nil {
		return nil, err
	}
	result := platformlogic.PlatformRespBase(response.Base)
	return &result, nil
}

func listPlatformWhiteLabelTemplateRevisions(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformWhiteLabelTemplateRevisionsReq) (*types.PlatformWhiteLabelTemplateRevisionListResp, error) {
	response, err := svcCtx.CoreCli.ListWhiteLabelTemplateRevisions(ctx, &core.WhiteLabelTemplateRevisionListReq{
		Page: platformlogic.PlatformPage(req.PageReq), TemplateId: req.Id, Status: core.WhiteLabelRevisionStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformWhiteLabelTemplateRevision, 0, len(response.Data))
	for _, item := range response.Data {
		data = append(data, platformlogic.MapPlatformWhiteLabelTemplateRevision(item))
	}
	return &types.PlatformWhiteLabelTemplateRevisionListResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: data}, nil
}

func publishPlatformWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, req *types.PublishPlatformWhiteLabelTemplateReq) (*types.PlatformWhiteLabelTemplateResp, error) {
	response, err := svcCtx.CoreCli.PublishWhiteLabelTemplate(ctx, &core.PublishWhiteLabelTemplateReq{Id: req.Id, Revision: req.Revision})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplate(response.Data)}, nil
}

func changePlatformWhiteLabelTemplateStatus(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ChangePlatformWhiteLabelTemplateStatusReq) (*types.PlatformWhiteLabelTemplateResp, error) {
	response, err := svcCtx.CoreCli.ChangeWhiteLabelTemplateStatus(ctx, &core.ChangeWhiteLabelTemplateStatusReq{Id: req.Id, Status: core.WhiteLabelTemplateStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelTemplateResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelTemplate(response.Data)}, nil
}

func createPlatformWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, req *types.CreatePlatformWhiteLabelProductReq) (*types.PlatformWhiteLabelProductResp, error) {
	parameterValues, err := sealWhiteLabelSensitiveParameters(ctx, svcCtx, req.TemplateId, req.ParameterValuesJson, "")
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.CoreCli.CreateWhiteLabelProduct(ctx, &core.CreateWhiteLabelProductReq{
		AppId: req.AppId, ProductCode: req.ProductCode, ProductName: req.ProductName,
		TemplateId: req.TemplateId, TemplateRevision: req.TemplateRevision, BrandingProfileId: req.BrandingProfileId,
		PackageName: req.PackageName, SigningConfigId: req.SigningConfigId, ParameterValuesJson: parameterValues,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelProductResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelProduct(response.Data)}, nil
}

func updatePlatformWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, req *types.UpdatePlatformWhiteLabelProductReq) (*types.PlatformWhiteLabelProductResp, error) {
	existing, err := svcCtx.CoreCli.GetWhiteLabelProduct(ctx, &core.WhiteLabelProductIdReq{Id: req.Id})
	if err != nil {
		return nil, err
	}
	if existing.Data == nil {
		return nil, status.Error(codes.NotFound, "white-label product not found")
	}
	templateID := req.TemplateId
	if templateID <= 0 {
		templateID = existing.Data.TemplateId
	}
	parameterValues, err := sealWhiteLabelSensitiveParameters(ctx, svcCtx, templateID, req.ParameterValuesJson, existing.Data.ParameterValuesJson)
	if err != nil {
		return nil, err
	}
	response, err := svcCtx.CoreCli.UpdateWhiteLabelProduct(ctx, &core.UpdateWhiteLabelProductReq{
		Id: req.Id, ProductName: req.ProductName, BrandingProfileId: req.BrandingProfileId,
		PackageName: req.PackageName, SigningConfigId: req.SigningConfigId, ParameterValuesJson: parameterValues,
		TemplateId: req.TemplateId, TemplateRevision: req.TemplateRevision,
	})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelProductResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelProduct(response.Data)}, nil
}

type whiteLabelParameterDefinition struct {
	Type      string `json:"type"`
	Sensitive bool   `json:"sensitive"`
}

type whiteLabelParameterSchema struct {
	Properties map[string]whiteLabelParameterDefinition `json:"properties"`
}

func sealWhiteLabelSensitiveParameters(ctx context.Context, svcCtx *svc.ServiceContext, templateID int64, raw, existingRaw string) (string, error) {
	template, err := svcCtx.CoreCli.GetWhiteLabelTemplate(ctx, &core.WhiteLabelTemplateIdReq{Id: templateID})
	if err != nil {
		return "", err
	}
	if template.Data == nil {
		return "", status.Error(codes.NotFound, "white-label template not found")
	}
	var schema whiteLabelParameterSchema
	if err := json.Unmarshal([]byte(template.Data.ParameterSchemaJson), &schema); err != nil {
		return "", status.Error(codes.FailedPrecondition, "white-label parameter schema is invalid")
	}
	values := make(map[string]any)
	normalizedRaw := strings.TrimSpace(raw)
	if normalizedRaw == "" {
		normalizedRaw = `{}`
	}
	if err := json.Unmarshal([]byte(normalizedRaw), &values); err != nil {
		return "", status.Error(codes.InvalidArgument, "parameterValuesJson must be a JSON object")
	}
	existing := make(map[string]any)
	if strings.TrimSpace(existingRaw) != "" {
		_ = json.Unmarshal([]byte(existingRaw), &existing)
	}
	for name, definition := range schema.Properties {
		if !definition.Sensitive {
			continue
		}
		value, exists := values[name]
		if !exists {
			continue
		}
		plaintext, ok := value.(string)
		if !ok {
			return "", status.Errorf(codes.InvalidArgument, "sensitive parameter %s must be a string", name)
		}
		if plaintext == "***" {
			previous, ok := existing[name].(string)
			if !ok || !secretbox.IsSealed(previous) {
				return "", status.Errorf(codes.InvalidArgument, "sensitive parameter %s must be entered", name)
			}
			values[name] = previous
			continue
		}
		if strings.TrimSpace(plaintext) == "" || secretbox.IsSealed(plaintext) {
			return "", status.Errorf(codes.InvalidArgument, "sensitive parameter %s is invalid", name)
		}
		sealed, err := svcCtx.Secrets.Seal(plaintext)
		if err != nil {
			return "", status.Errorf(codes.Internal, "encrypt sensitive parameter %s failed: %v", name, err)
		}
		values[name] = sealed
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", status.Error(codes.InvalidArgument, "parameterValuesJson cannot be encoded")
	}
	return string(encoded), nil
}

func deletePlatformWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.RespBase, error) {
	response, err := svcCtx.CoreCli.DeleteWhiteLabelProduct(ctx, &core.WhiteLabelProductIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	result := platformlogic.PlatformRespBase(response.Base)
	return &result, nil
}

func getPlatformWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.PlatformWhiteLabelProductResp, error) {
	response, err := svcCtx.CoreCli.GetWhiteLabelProduct(ctx, &core.WhiteLabelProductIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelProductResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelProduct(response.Data)}, nil
}

func listPlatformWhiteLabelProducts(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ListPlatformWhiteLabelProductsReq) (*types.PlatformWhiteLabelProductListResp, error) {
	response, err := svcCtx.CoreCli.ListWhiteLabelProducts(ctx, &core.WhiteLabelProductListReq{
		Page: platformlogic.PlatformPage(req.PageReq), AppId: req.AppId, Keyword: req.Keyword,
		Status: core.WhiteLabelProductStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}
	data := make([]types.PlatformWhiteLabelProduct, 0, len(response.Data))
	for _, item := range response.Data {
		data = append(data, platformlogic.MapPlatformWhiteLabelProduct(item))
	}
	return &types.PlatformWhiteLabelProductListResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: data}, nil
}

func changePlatformWhiteLabelProductStatus(ctx context.Context, svcCtx *svc.ServiceContext, req *types.ChangePlatformWhiteLabelProductStatusReq) (*types.PlatformWhiteLabelProductResp, error) {
	response, err := svcCtx.CoreCli.ChangeWhiteLabelProductStatus(ctx, &core.ChangeWhiteLabelProductStatusReq{Id: req.Id, Status: core.WhiteLabelProductStatus(req.Status)})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelProductResp{RespBase: platformlogic.PlatformRespBase(response.Base), Data: platformlogic.MapPlatformWhiteLabelProduct(response.Data)}, nil
}

func preflightPlatformWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, id int64) (*types.PlatformWhiteLabelProductPreflightResp, error) {
	response, err := svcCtx.CoreCli.PreflightWhiteLabelProduct(ctx, &core.WhiteLabelProductIdReq{Id: id})
	if err != nil {
		return nil, err
	}
	return &types.PlatformWhiteLabelProductPreflightResp{RespBase: platformlogic.PlatformRespBase(response.Base), Compatible: response.Compatible, ReportJson: response.ReportJson}, nil
}
