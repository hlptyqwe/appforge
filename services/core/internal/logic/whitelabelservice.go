package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	whiteLabelTemplateSelect = `SELECT id, tenant_id, app_id, template_code, template_name, source_version_id,
parameter_schema, compatibility_rules, status, published_revision, create_by, create_time, update_time
FROM t_white_label_template`
	whiteLabelRevisionSelect = `SELECT id, tenant_id, template_id, revision, package_name_rule, manifest_patch,
resource_patch, extension_files, expected_artifacts, checksum, status, create_by, create_time
FROM t_white_label_template_revision`
	whiteLabelProductSelect = `SELECT id, tenant_id, app_id, product_code, product_name, template_id,
template_revision, branding_profile_id, package_name, signing_config_id, parameter_values, status,
create_by, create_time, update_time FROM t_white_label_product`
)

var (
	androidPackagePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	codePattern           = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)
	resourceNamePattern   = regexp.MustCompile(`^[a-z0-9_.]+$`)
	manifestNamePattern   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9:_-]*$`)
	parameterTokenPattern = regexp.MustCompile(`\{\{parameters\.([a-zA-Z0-9_.-]+)\}\}`)
)

type whiteLabelDependency struct {
	Template *models.TWhiteLabelTemplate
	Revision *models.TWhiteLabelTemplateRevision
	Branding *models.TAppBrandingProfile
	Signing  *models.TAppSigningConfig
}

type whiteLabelPreflightReport struct {
	Compatible bool                       `json:"compatible"`
	Checks     []whiteLabelPreflightCheck `json:"checks"`
}

type whiteLabelPreflightCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type templateFileOperation struct {
	Op               string          `json:"op"`
	Path             string          `json:"path"`
	ObjectID         int64           `json:"objectId"`
	Content          json.RawMessage `json:"content"`
	ContentParameter string          `json:"contentParameter"`
}

type templateFileBinding struct {
	Operation string
	Path      string
	ObjectID  int64
}

// whiteLabelBuildSnapshot 是传给Builder的不可变V3构建契约。
type whiteLabelBuildSnapshot struct {
	ProductID             int64  `json:"productId"`
	ProductCode           string `json:"productCode"`
	ProductName           string `json:"productName"`
	TemplateID            int64  `json:"templateId"`
	TemplateCode          string `json:"templateCode"`
	TemplateRevision      int64  `json:"templateRevision"`
	TemplateChecksum      string `json:"templateChecksum"`
	OriginalPackageName   string `json:"originalPackageName"`
	TargetPackageName     string `json:"targetPackageName"`
	PackageNameRuleJSON   string `json:"packageNameRuleJson"`
	ManifestPatchJSON     string `json:"manifestPatchJson"`
	ResourcePatchJSON     string `json:"resourcePatchJson"`
	ExtensionFilesJSON    string `json:"extensionFilesJson"`
	ExpectedArtifactsJSON string `json:"expectedArtifactsJson"`
	ParameterValuesJSON   string `json:"parameterValuesJson"`
	CertificateSHA256     string `json:"certificateSha256"`
}

func prepareWhiteLabelBuildSnapshot(ctx context.Context, svcCtx *svc.ServiceContext, tenant, appID, versionID, productID int64, originalPackageName string) (*models.TWhiteLabelProduct, *whiteLabelDependency, string, error) {
	if productID <= 0 {
		return nil, nil, "", nil
	}
	product, err := svcCtx.WhiteLabelProductModel.FindOne(ctx, productID)
	if err != nil || product.TenantId != tenant || product.AppId != appID {
		return nil, nil, "", status.Error(codes.InvalidArgument, "white-label product is invalid")
	}
	if product.Status != int64(core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_ENABLED) {
		return nil, nil, "", status.Error(codes.FailedPrecondition, "white-label product must be enabled")
	}
	dependency, err := loadWhiteLabelDependencies(ctx, svcCtx, tenant, appID, product.TemplateId, int32(product.TemplateRevision), product.BrandingProfileId, product.SigningConfigId)
	if err != nil {
		return nil, nil, "", err
	}
	if dependency.Template.Status != int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_PUBLISHED) ||
		dependency.Template.PublishedRevision != product.TemplateRevision ||
		dependency.Revision.Status != int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_PUBLISHED) {
		return nil, nil, "", status.Error(codes.FailedPrecondition, "white-label template revision is not currently published")
	}
	if dependency.Template.SourceVersionId != versionID {
		return nil, nil, "", status.Error(codes.InvalidArgument, "version_id must match the white-label template source version")
	}
	if dependency.Branding.Status != int64(core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_ENABLED) || dependency.Signing.Status != 1 {
		return nil, nil, "", status.Error(codes.FailedPrecondition, "white-label branding and signing configurations must be enabled")
	}
	if err := ensurePackageCertificate(ctx, svcCtx, tenant, product.PackageName, dependency.Signing); err != nil {
		return nil, nil, "", err
	}
	preflight, err := preflightWhiteLabelProduct(ctx, svcCtx, &core.WhiteLabelProductIdReq{Id: product.Id})
	if err != nil {
		return nil, nil, "", err
	}
	if !preflight.Compatible {
		return nil, nil, "", status.Errorf(codes.FailedPrecondition, "white-label product preflight failed: %s", preflight.ReportJson)
	}
	snapshot := whiteLabelBuildSnapshot{
		ProductID: product.Id, ProductCode: product.ProductCode, ProductName: product.ProductName,
		TemplateID: dependency.Template.Id, TemplateCode: dependency.Template.TemplateCode,
		TemplateRevision: dependency.Revision.Revision, TemplateChecksum: dependency.Revision.Checksum,
		OriginalPackageName: originalPackageName, TargetPackageName: product.PackageName,
		PackageNameRuleJSON: dependency.Revision.PackageNameRule,
		ManifestPatchJSON:   stringValue(dependency.Revision.ManifestPatch), ResourcePatchJSON: stringValue(dependency.Revision.ResourcePatch),
		ExtensionFilesJSON: stringValue(dependency.Revision.ExtensionFiles), ExpectedArtifactsJSON: stringValue(dependency.Revision.ExpectedArtifacts),
		ParameterValuesJSON: stringValue(product.ParameterValues), CertificateSHA256: strings.ToLower(stringValue(dependency.Signing.CertificateSha256)),
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, nil, "", status.Errorf(codes.Internal, "encode white-label build snapshot failed: %v", err)
	}
	return product, dependency, string(encoded), nil
}

func parseWhiteLabelBuildSnapshot(raw string) (*whiteLabelBuildSnapshot, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var snapshot whiteLabelBuildSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "white-label build snapshot is invalid: %v", err)
	}
	if snapshot.ProductID <= 0 || snapshot.TemplateID <= 0 || snapshot.TemplateRevision <= 0 || len(snapshot.TemplateChecksum) != 64 {
		return nil, status.Error(codes.FailedPrecondition, "white-label build snapshot is incomplete")
	}
	if _, err := validatePackageName(snapshot.TargetPackageName); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "white-label build snapshot package name is invalid")
	}
	return &snapshot, nil
}

func mapWhiteLabelTemplate(item *models.TWhiteLabelTemplate) *core.WhiteLabelTemplate {
	if item == nil {
		return nil
	}
	return &core.WhiteLabelTemplate{
		Id: item.Id, TenantId: item.TenantId, AppId: item.AppId,
		TemplateCode: item.TemplateCode, TemplateName: item.TemplateName,
		SourceVersionId: item.SourceVersionId, ParameterSchemaJson: stringValue(item.ParameterSchema),
		CompatibilityRulesJson: stringValue(item.CompatibilityRules),
		Status:                 core.WhiteLabelTemplateStatus(item.Status), PublishedRevision: int32(item.PublishedRevision),
		CreateBy: item.CreateBy, CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func mapWhiteLabelRevision(item *models.TWhiteLabelTemplateRevision) *core.WhiteLabelTemplateRevision {
	if item == nil {
		return nil
	}
	return &core.WhiteLabelTemplateRevision{
		Id: item.Id, TenantId: item.TenantId, TemplateId: item.TemplateId, Revision: int32(item.Revision),
		PackageNameRuleJson: item.PackageNameRule, ManifestPatchJson: stringValue(item.ManifestPatch),
		ResourcePatchJson: stringValue(item.ResourcePatch), ExtensionFilesJson: stringValue(item.ExtensionFiles),
		ExpectedArtifactsJson: stringValue(item.ExpectedArtifacts), Checksum: item.Checksum,
		Status: core.WhiteLabelRevisionStatus(item.Status), CreateBy: item.CreateBy, CreateTime: millis(item.CreateTime),
	}
}

func mapWhiteLabelProduct(item *models.TWhiteLabelProduct) *core.WhiteLabelProduct {
	if item == nil {
		return nil
	}
	return &core.WhiteLabelProduct{
		Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, ProductCode: item.ProductCode,
		ProductName: item.ProductName, TemplateId: item.TemplateId, TemplateRevision: int32(item.TemplateRevision),
		BrandingProfileId: item.BrandingProfileId, PackageName: item.PackageName,
		SigningConfigId: item.SigningConfigId, ParameterValuesJson: stringValue(item.ParameterValues),
		Status: core.WhiteLabelProductStatus(item.Status), CreateBy: item.CreateBy,
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func normalizeJSON(raw, field, empty string) (string, any, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = empty
	}
	var parsed any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return "", nil, status.Errorf(codes.InvalidArgument, "%s must be valid JSON: %v", field, err)
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return "", nil, status.Errorf(codes.InvalidArgument, "%s cannot be normalized: %v", field, err)
	}
	return string(encoded), parsed, nil
}

func validateJSONDocument(raw, field, empty string) (string, error) {
	normalized, _, err := normalizeJSON(raw, field, empty)
	return normalized, err
}

func validatePackageName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > 255 || !androidPackagePattern.MatchString(value) {
		return "", status.Error(codes.InvalidArgument, "package_name is not a valid Android applicationId")
	}
	return value, nil
}

func validatePatchDocument(raw, field string, allowed map[string]struct{}) (string, error) {
	normalized, parsed, err := normalizeJSON(raw, field, `[]`)
	if err != nil {
		return "", err
	}
	operations, ok := parsed.([]any)
	if !ok {
		return "", status.Errorf(codes.InvalidArgument, "%s must be a JSON array", field)
	}
	if len(operations) > 100 {
		return "", status.Errorf(codes.InvalidArgument, "%s has too many operations", field)
	}
	for index, value := range operations {
		operation, ok := value.(map[string]any)
		if !ok {
			return "", status.Errorf(codes.InvalidArgument, "%s[%d] must be an object", field, index)
		}
		op, ok := operation["op"].(string)
		if !ok {
			return "", status.Errorf(codes.InvalidArgument, "%s[%d].op is required", field, index)
		}
		if _, ok := allowed[op]; !ok {
			return "", status.Errorf(codes.InvalidArgument, "%s[%d].op is not allowed", field, index)
		}
		if err := validatePatchOperation(operation, field, index); err != nil {
			return "", err
		}
		if err := validateControlledValue(operation, field); err != nil {
			return "", err
		}
		if err := validateNoEmbeddedSecrets(operation, field); err != nil {
			return "", err
		}
	}
	return normalized, nil
}

func validatePatchOperation(operation map[string]any, field string, index int) error {
	op, _ := operation["op"].(string)
	allowedFields := map[string]struct{}{"op": {}}
	requireString := func(name string) (string, error) {
		value, ok := operation[name].(string)
		if !ok || strings.TrimSpace(value) == "" || len(value) > 2048 {
			return "", status.Errorf(codes.InvalidArgument, "%s[%d].%s is required", field, index, name)
		}
		return value, nil
	}
	requireOneOf := func(first, second string) error {
		_, hasFirst := operation[first]
		secondValue, hasSecond := operation[second]
		if hasSecond {
			if text, ok := secondValue.(string); !ok || strings.TrimSpace(text) == "" {
				return status.Errorf(codes.InvalidArgument, "%s[%d].%s must be a non-empty string", field, index, second)
			}
		}
		if hasFirst == hasSecond {
			return status.Errorf(codes.InvalidArgument, "%s[%d] must contain exactly one of %s or %s", field, index, first, second)
		}
		return nil
	}

	switch op {
	case "manifest.setPackage":
	case "manifest.replaceProviderAuthority", "manifest.replaceIntentScheme", "manifest.replaceAppLinkHost":
		allowedFields["old"], allowedFields["new"] = struct{}{}, struct{}{}
		if _, err := requireString("old"); err != nil {
			return err
		}
		if _, err := requireString("new"); err != nil {
			return err
		}
	case "manifest.setAttribute":
		allowedFields["target"], allowedFields["name"] = struct{}{}, struct{}{}
		allowedFields["value"], allowedFields["valueParameter"] = struct{}{}, struct{}{}
		target, err := requireString("target")
		if err != nil || target != "application" {
			return status.Errorf(codes.InvalidArgument, "%s[%d].target must be application", field, index)
		}
		name, err := requireString("name")
		if err != nil || !manifestNamePattern.MatchString(name) {
			return status.Errorf(codes.InvalidArgument, "%s[%d].name is invalid", field, index)
		}
		if err := requireOneOf("value", "valueParameter"); err != nil {
			return err
		}
	case "resource.replaceString":
		allowedFields["name"], allowedFields["value"] = struct{}{}, struct{}{}
		allowedFields["valueParameter"] = struct{}{}
		name, err := requireString("name")
		if err != nil || !resourceNamePattern.MatchString(name) {
			return status.Errorf(codes.InvalidArgument, "%s[%d].name is invalid", field, index)
		}
		if err := requireOneOf("value", "valueParameter"); err != nil {
			return err
		}
	case "resource.replaceFile":
		allowedFields["path"], allowedFields["objectId"] = struct{}{}, struct{}{}
		pathValue, err := requireString("path")
		if err != nil {
			return err
		}
		if _, err := validateTemplateTarget(op, pathValue); err != nil {
			return status.Errorf(codes.InvalidArgument, "%s[%d]: %v", field, index, err)
		}
		if objectID, ok := operation["objectId"].(float64); !ok || objectID <= 0 || objectID != float64(int64(objectID)) {
			return status.Errorf(codes.InvalidArgument, "%s[%d].objectId is required", field, index)
		}
	case "asset.writeJson":
		allowedFields["path"], allowedFields["content"] = struct{}{}, struct{}{}
		allowedFields["contentParameter"] = struct{}{}
		pathValue, err := requireString("path")
		if err != nil || !strings.HasPrefix(pathValue, "assets/") || strings.ToLower(path.Ext(pathValue)) != ".json" {
			return status.Errorf(codes.InvalidArgument, "%s[%d].path must be a JSON file below assets/", field, index)
		}
		if _, err := validateTemplateTarget("extension.writeValidatedFile", pathValue); err != nil {
			return status.Errorf(codes.InvalidArgument, "%s[%d]: %v", field, index, err)
		}
		if err := requireOneOf("content", "contentParameter"); err != nil {
			return err
		}
	case "extension.writeValidatedFile":
		allowedFields["path"], allowedFields["objectId"] = struct{}{}, struct{}{}
		allowedFields["content"], allowedFields["contentParameter"], allowedFields["value"] = struct{}{}, struct{}{}, struct{}{}
		pathValue, err := requireString("path")
		if err != nil {
			return err
		}
		if _, err := validateTemplateTarget(op, pathValue); err != nil {
			return status.Errorf(codes.InvalidArgument, "%s[%d]: %v", field, index, err)
		}
		sources := 0
		for _, name := range []string{"objectId", "content", "contentParameter", "value"} {
			if _, exists := operation[name]; exists {
				sources++
			}
		}
		if sources != 1 {
			return status.Errorf(codes.InvalidArgument, "%s[%d] must contain exactly one content source", field, index)
		}
		if value, exists := operation["contentParameter"]; exists {
			if text, ok := value.(string); !ok || strings.TrimSpace(text) == "" {
				return status.Errorf(codes.InvalidArgument, "%s[%d].contentParameter must be a non-empty string", field, index)
			}
		}
		if value, exists := operation["objectId"]; exists {
			objectID, ok := value.(float64)
			if !ok || objectID <= 0 || objectID != float64(int64(objectID)) {
				return status.Errorf(codes.InvalidArgument, "%s[%d].objectId is invalid", field, index)
			}
		}
	}
	for name := range operation {
		if _, ok := allowedFields[name]; !ok {
			return status.Errorf(codes.InvalidArgument, "%s[%d] contains unsupported field %s", field, index, name)
		}
	}
	return nil
}

func validateRevisionParameterReferences(schemaRaw string, documents map[string]string) error {
	_, schemaValue, err := normalizeJSON(schemaRaw, "parameter_schema_json", `{"type":"object","properties":{},"additionalProperties":false}`)
	if err != nil {
		return err
	}
	schema, _ := schemaValue.(map[string]any)
	properties, _ := schema["properties"].(map[string]any)
	for documentName, raw := range documents {
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return status.Errorf(codes.InvalidArgument, "%s is invalid", documentName)
		}
		if err := walkRevisionParameterReferences(value, properties, documentName); err != nil {
			return err
		}
	}
	return nil
}

func walkRevisionParameterReferences(value any, properties map[string]any, field string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "valueParameter" || key == "contentParameter" {
				name, ok := child.(string)
				if !ok || properties[name] == nil {
					return status.Errorf(codes.InvalidArgument, "%s references undeclared parameter %v", field, child)
				}
			}
			if err := walkRevisionParameterReferences(child, properties, field); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := walkRevisionParameterReferences(child, properties, field); err != nil {
				return err
			}
		}
	case string:
		for _, match := range parameterTokenPattern.FindAllStringSubmatch(typed, -1) {
			if len(match) < 2 {
				return status.Errorf(codes.InvalidArgument, "%s contains an invalid parameter token", field)
			}
			if properties[match[1]] == nil {
				return status.Errorf(codes.InvalidArgument, "%s references undeclared parameter %s", field, match[1])
			}
		}
	}
	return nil
}

func validateNoEmbeddedSecrets(value any, field string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
			if normalized == "clientsecret" || normalized == "privatekey" || normalized == "password" ||
				normalized == "accesstoken" || normalized == "refreshtoken" {
				return status.Errorf(codes.InvalidArgument, "%s contains plaintext sensitive field %s; use a sensitive parameter", field, key)
			}
			if err := validateNoEmbeddedSecrets(child, field); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := validateNoEmbeddedSecrets(child, field); err != nil {
				return err
			}
		}
	case string:
		if strings.Contains(strings.ToLower(typed), "-----begin private key") || strings.Contains(strings.ToLower(typed), "-----begin rsa private key") {
			return status.Errorf(codes.InvalidArgument, "%s contains plaintext private key material", field)
		}
	}
	return nil
}

func validateControlledValue(value any, field string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "command") || strings.Contains(lower, "script") ||
				strings.Contains(lower, "executable") || strings.Contains(lower, "download") || lower == "url" {
				return status.Errorf(codes.InvalidArgument, "%s contains forbidden field %s", field, key)
			}
			if strings.Contains(lower, "path") || lower == "target" || lower == "file" {
				if pathValue, ok := child.(string); ok && (strings.HasPrefix(pathValue, "/") || strings.Contains(pathValue, "..") || strings.Contains(pathValue, `\\`)) {
					return status.Errorf(codes.InvalidArgument, "%s contains unsafe path", field)
				}
			}
			if err := validateControlledValue(child, field); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := validateControlledValue(child, field); err != nil {
				return err
			}
		}
	case string:
		if len(typed) > 256*1024 {
			return status.Errorf(codes.InvalidArgument, "%s contains an oversized value", field)
		}
	}
	return nil
}

func validateRevisionDocuments(in *core.CreateWhiteLabelTemplateRevisionReq) (map[string]string, string, error) {
	packageRule, packageRuleValue, err := normalizeJSON(in.PackageNameRuleJson, "package_name_rule_json", `{}`)
	if err != nil {
		return nil, "", err
	}
	if _, ok := packageRuleValue.(map[string]any); !ok {
		return nil, "", status.Error(codes.InvalidArgument, "package_name_rule_json must be an object")
	}
	if err := validateControlledValue(packageRuleValue, "package_name_rule_json"); err != nil {
		return nil, "", err
	}
	manifest, err := validatePatchDocument(in.ManifestPatchJson, "manifest_patch_json", map[string]struct{}{
		"manifest.setPackage": {}, "manifest.setAttribute": {},
		"manifest.replaceProviderAuthority": {}, "manifest.replaceIntentScheme": {},
		"manifest.replaceAppLinkHost": {},
	})
	if err != nil {
		return nil, "", err
	}
	resources, err := validatePatchDocument(in.ResourcePatchJson, "resource_patch_json", map[string]struct{}{
		"resource.replaceString": {}, "resource.replaceFile": {}, "asset.writeJson": {},
	})
	if err != nil {
		return nil, "", err
	}
	extensions, err := validatePatchDocument(in.ExtensionFilesJson, "extension_files_json", map[string]struct{}{
		"extension.writeValidatedFile": {},
	})
	if err != nil {
		return nil, "", err
	}
	if _, err := extractTemplateFileBindings(resources, extensions); err != nil {
		return nil, "", err
	}
	expected, err := validateJSONDocument(in.ExpectedArtifactsJson, "expected_artifacts_json", `{}`)
	if err != nil {
		return nil, "", err
	}
	documents := map[string]string{
		"packageNameRule": packageRule, "manifestPatch": manifest, "resourcePatch": resources,
		"extensionFiles": extensions, "expectedArtifacts": expected,
	}
	canonical, _ := json.Marshal(documents)
	sum := sha256.Sum256(canonical)
	return documents, hex.EncodeToString(sum[:]), nil
}

func extractTemplateFileBindings(resourcePatch, extensionFiles string) ([]templateFileBinding, error) {
	bindings := make([]templateFileBinding, 0)
	for _, document := range []struct {
		name string
		raw  string
	}{
		{name: "resource_patch_json", raw: resourcePatch},
		{name: "extension_files_json", raw: extensionFiles},
	} {
		var operations []templateFileOperation
		if err := json.Unmarshal([]byte(document.raw), &operations); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "%s must be valid JSON: %v", document.name, err)
		}
		for index, operation := range operations {
			if operation.Op != "resource.replaceFile" && operation.Op != "extension.writeValidatedFile" {
				continue
			}
			target, err := validateTemplateTarget(operation.Op, operation.Path)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "%s[%d]: %v", document.name, index, err)
			}
			if operation.Op == "resource.replaceFile" && operation.ObjectID <= 0 {
				return nil, status.Errorf(codes.InvalidArgument, "%s[%d].objectId is required", document.name, index)
			}
			if operation.Op == "extension.writeValidatedFile" && operation.ObjectID <= 0 &&
				operation.ContentParameter == "" && len(operation.Content) == 0 {
				return nil, status.Errorf(codes.InvalidArgument, "%s[%d] requires objectId, contentParameter or content", document.name, index)
			}
			if operation.ObjectID > 0 {
				bindings = append(bindings, templateFileBinding{Operation: operation.Op, Path: target, ObjectID: operation.ObjectID})
			}
		}
	}
	return bindings, nil
}

func validateTemplateTarget(operation, target string) (string, error) {
	target = strings.TrimSpace(strings.ReplaceAll(target, `\`, "/"))
	clean := path.Clean(target)
	if target == "" || clean != target || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", status.Error(codes.InvalidArgument, "template file path is unsafe")
	}
	extension := strings.ToLower(path.Ext(clean))
	switch operation {
	case "resource.replaceFile":
		if !strings.HasPrefix(clean, "res/") || (extension != ".json" && extension != ".xml" && extension != ".txt" && extension != ".png" && extension != ".webp") {
			return "", status.Error(codes.InvalidArgument, "resource replacement must target a supported file below res/")
		}
	case "extension.writeValidatedFile":
		if (!strings.HasPrefix(clean, "assets/") && !strings.HasPrefix(clean, "res/raw/")) ||
			(extension != ".json" && extension != ".xml" && extension != ".txt") {
			return "", status.Error(codes.InvalidArgument, "extension file must target JSON, XML or TXT below assets/ or res/raw/")
		}
	default:
		return "", status.Error(codes.InvalidArgument, "unsupported template file operation")
	}
	return clean, nil
}

func bindTemplateFiles(ctx context.Context, svcCtx *svc.ServiceContext, template *models.TWhiteLabelTemplate, bindings []templateFileBinding) error {
	seen := make(map[int64]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, ok := seen[binding.ObjectID]; ok {
			continue
		}
		item, err := svcCtx.StorageObjectModel.FindOne(ctx, binding.ObjectID)
		if err != nil || item.TenantId != template.TenantId || item.AppId != template.AppId ||
			item.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_TEMPLATE_FILE) ||
			(item.Status != storageStatusReady && item.Status != storageStatusBound) {
			return status.Errorf(codes.FailedPrecondition, "template file object %d is unavailable", binding.ObjectID)
		}
		if !strings.EqualFold(path.Ext(item.OriginalName), path.Ext(binding.Path)) {
			return status.Errorf(codes.InvalidArgument, "template file object %d extension does not match target", binding.ObjectID)
		}
		if item.Status != storageStatusBound {
			item.Status = storageStatusBound
			if err := svcCtx.StorageObjectModel.Update(ctx, item); err != nil {
				return status.Errorf(codes.Internal, "bind template file object %d failed: %v", binding.ObjectID, err)
			}
		}
		seen[binding.ObjectID] = struct{}{}
	}
	return nil
}

func templateFileObjectIDs(snapshot *whiteLabelBuildSnapshot) ([]int64, error) {
	if snapshot == nil {
		return nil, nil
	}
	bindings, err := extractTemplateFileBindings(snapshot.ResourcePatchJSON, snapshot.ExtensionFilesJSON)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "white-label template file bindings are invalid")
	}
	ids := make([]int64, 0, len(bindings))
	seen := make(map[int64]struct{}, len(bindings))
	for _, binding := range bindings {
		if _, ok := seen[binding.ObjectID]; ok {
			continue
		}
		seen[binding.ObjectID] = struct{}{}
		ids = append(ids, binding.ObjectID)
	}
	return ids, nil
}

func validateParameterValues(schemaRaw, valuesRaw string) (string, error) {
	valuesJSON, valuesValue, err := normalizeJSON(valuesRaw, "parameter_values_json", `{}`)
	if err != nil {
		return "", err
	}
	values, ok := valuesValue.(map[string]any)
	if !ok {
		return "", status.Error(codes.InvalidArgument, "parameter_values_json must be an object")
	}
	_, schemaValue, err := normalizeJSON(schemaRaw, "parameter_schema_json", `{"type":"object","properties":{},"additionalProperties":false}`)
	if err != nil {
		return "", err
	}
	schema, ok := schemaValue.(map[string]any)
	if !ok || schema["type"] != "object" {
		return "", status.Error(codes.FailedPrecondition, "template parameter schema must describe an object")
	}
	properties, _ := schema["properties"].(map[string]any)
	if additional, exists := schema["additionalProperties"]; !exists || additional != false {
		return "", status.Error(codes.FailedPrecondition, "template parameter schema must set additionalProperties to false")
	}
	for key, value := range values {
		definition, exists := properties[key]
		if !exists {
			return "", status.Errorf(codes.InvalidArgument, "parameter %s is not declared by template schema", key)
		}
		definitionObject, _ := definition.(map[string]any)
		if expected, _ := definitionObject["type"].(string); expected != "" && !matchesJSONType(value, expected) {
			return "", status.Errorf(codes.InvalidArgument, "parameter %s must be %s", key, expected)
		}
	}
	if required, ok := schema["required"].([]any); ok {
		for _, item := range required {
			name, _ := item.(string)
			if _, exists := values[name]; name != "" && !exists {
				return "", status.Errorf(codes.InvalidArgument, "parameter %s is required", name)
			}
		}
	}
	if err := validateControlledValue(values, "parameter_values_json"); err != nil {
		return "", err
	}
	return valuesJSON, nil
}

func validateParameterSchema(schemaRaw string) (string, error) {
	schemaJSON, schemaValue, err := normalizeJSON(schemaRaw, "parameter_schema_json", `{"type":"object","properties":{},"additionalProperties":false}`)
	if err != nil {
		return "", err
	}
	schema, ok := schemaValue.(map[string]any)
	if !ok || schema["type"] != "object" {
		return "", status.Error(codes.InvalidArgument, "parameter_schema_json must describe an object")
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return "", status.Error(codes.InvalidArgument, "parameter_schema_json.properties must be an object")
	}
	if additional, exists := schema["additionalProperties"]; !exists || additional != false {
		return "", status.Error(codes.InvalidArgument, "parameter_schema_json must set additionalProperties to false")
	}
	allowedTypes := map[string]struct{}{"string": {}, "number": {}, "integer": {}, "boolean": {}, "object": {}, "array": {}, "null": {}}
	for name, value := range properties {
		definition, ok := value.(map[string]any)
		if !ok {
			return "", status.Errorf(codes.InvalidArgument, "parameter %s schema must be an object", name)
		}
		parameterType, _ := definition["type"].(string)
		if _, ok := allowedTypes[parameterType]; !ok {
			return "", status.Errorf(codes.InvalidArgument, "parameter %s has unsupported type", name)
		}
		if sensitive, exists := definition["sensitive"]; exists {
			flag, ok := sensitive.(bool)
			if !ok {
				return "", status.Errorf(codes.InvalidArgument, "parameter %s sensitive flag must be boolean", name)
			}
			if flag && parameterType != "string" {
				return "", status.Errorf(codes.InvalidArgument, "sensitive parameter %s must use string type", name)
			}
		}
	}
	if required, ok := schema["required"].([]any); ok {
		for _, value := range required {
			name, ok := value.(string)
			if !ok || name == "" {
				return "", status.Error(codes.InvalidArgument, "parameter_schema_json.required must contain names")
			}
			if _, exists := properties[name]; !exists {
				return "", status.Errorf(codes.InvalidArgument, "required parameter %s is not declared", name)
			}
		}
	}
	return schemaJSON, nil
}

func matchesJSONType(value any, expected string) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "number", "integer":
		_, ok := value.(float64)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "null":
		return value == nil
	default:
		return false
	}
}

func createWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateWhiteLabelTemplateReq) (*core.WhiteLabelTemplateResp, error) {
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
	if err := requirePositive(in.SourceVersionId, "source_version_id"); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(in.TemplateCode)
	if !codePattern.MatchString(code) {
		return nil, status.Error(codes.InvalidArgument, "template_code must use 2-64 lowercase letters, digits, underscores or hyphens")
	}
	if err := requireText(in.TemplateName, "template_name", 128); err != nil {
		return nil, err
	}
	app, err := svcCtx.ApplicationModel.FindOne(ctx, in.AppId)
	if err != nil || app.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "application not found")
	}
	version, err := svcCtx.VersionModel.FindOne(ctx, in.SourceVersionId)
	if err != nil || version.TenantId != tenant || version.AppId != in.AppId || version.SourceApkObjectId <= 0 {
		return nil, status.Error(codes.FailedPrecondition, "source version is unavailable")
	}
	schema, err := validateParameterSchema(in.ParameterSchemaJson)
	if err != nil {
		return nil, err
	}
	rules, err := validateJSONDocument(in.CompatibilityRulesJson, "compatibility_rules_json", `{}`)
	if err != nil {
		return nil, err
	}
	if existing, findErr := svcCtx.WhiteLabelTemplateModel.FindOneByTenantIdTemplateCode(ctx, tenant, code); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "template_code already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check template_code failed: %v", findErr)
	}
	result, err := svcCtx.WhiteLabelTemplateModel.Insert(ctx, &models.TWhiteLabelTemplate{
		TenantId: tenant, AppId: in.AppId, TemplateCode: code, TemplateName: strings.TrimSpace(in.TemplateName),
		SourceVersionId: in.SourceVersionId, ParameterSchema: nullString(schema), CompatibilityRules: nullString(rules),
		Status: int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DRAFT), CreateBy: actorID(ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create white-label template failed: %v", err)
	}
	id, _ := result.LastInsertId()
	item, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label template")
	}
	return &core.WhiteLabelTemplateResp{Base: okBase(), Data: mapWhiteLabelTemplate(item)}, nil
}

func getWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelTemplateIdReq) (*core.WhiteLabelTemplateResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	item, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label template")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}
	return &core.WhiteLabelTemplateResp{Base: okBase(), Data: mapWhiteLabelTemplate(item)}, nil
}

func updateWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, in *core.UpdateWhiteLabelTemplateReq) (*core.WhiteLabelTemplateResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	item, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template not found")
	}
	if item.Status != int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DRAFT) || item.PublishedRevision != 0 {
		return nil, status.Error(codes.FailedPrecondition, "only an unpublished draft template can be updated")
	}
	if err := requireText(in.TemplateName, "template_name", 128); err != nil {
		return nil, err
	}
	version, err := svcCtx.VersionModel.FindOne(ctx, in.SourceVersionId)
	if err != nil || version.TenantId != tenant || version.AppId != item.AppId || version.SourceApkObjectId <= 0 {
		return nil, status.Error(codes.FailedPrecondition, "source version is unavailable")
	}
	schema, err := validateParameterSchema(in.ParameterSchemaJson)
	if err != nil {
		return nil, err
	}
	rules, err := validateJSONDocument(in.CompatibilityRulesJson, "compatibility_rules_json", `{}`)
	if err != nil {
		return nil, err
	}
	item.TemplateName = strings.TrimSpace(in.TemplateName)
	item.SourceVersionId = in.SourceVersionId
	item.ParameterSchema = nullString(schema)
	item.CompatibilityRules = nullString(rules)
	if err := svcCtx.WhiteLabelTemplateModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update white-label template failed: %v", err)
	}
	item, err = svcCtx.WhiteLabelTemplateModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label template")
	}
	return &core.WhiteLabelTemplateResp{Base: okBase(), Data: mapWhiteLabelTemplate(item)}, nil
}

func copyWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CopyWhiteLabelTemplateReq) (*core.WhiteLabelTemplateResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "source template id is required")
	}
	source, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.Id)
	if err != nil || source.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "source white-label template not found")
	}
	code := strings.TrimSpace(in.TemplateCode)
	if !codePattern.MatchString(code) {
		return nil, status.Error(codes.InvalidArgument, "template_code must use 2-64 lowercase letters, digits, underscores or hyphens")
	}
	if err := requireText(in.TemplateName, "template_name", 128); err != nil {
		return nil, err
	}
	if _, findErr := svcCtx.WhiteLabelTemplateModel.FindOneByTenantIdTemplateCode(ctx, tenant, code); findErr == nil {
		return nil, status.Error(codes.AlreadyExists, "template_code already exists")
	} else if findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check template_code failed: %v", findErr)
	}
	sourceVersionID := in.SourceVersionId
	if sourceVersionID <= 0 {
		sourceVersionID = source.SourceVersionId
	}
	version, err := svcCtx.VersionModel.FindOne(ctx, sourceVersionID)
	if err != nil || version.TenantId != tenant || version.AppId != source.AppId || version.SourceApkObjectId <= 0 {
		return nil, status.Error(codes.FailedPrecondition, "source version is unavailable")
	}
	result, err := svcCtx.WhiteLabelTemplateModel.Insert(ctx, &models.TWhiteLabelTemplate{
		TenantId: tenant, AppId: source.AppId, TemplateCode: code, TemplateName: strings.TrimSpace(in.TemplateName),
		SourceVersionId: sourceVersionID, ParameterSchema: source.ParameterSchema, CompatibilityRules: source.CompatibilityRules,
		Status: int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DRAFT), CreateBy: actorID(ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "copy white-label template failed: %v", err)
	}
	newID, _ := result.LastInsertId()
	var revisions []models.TWhiteLabelTemplateRevision
	if err := svcCtx.DB.QueryRowsCtx(ctx, &revisions, whiteLabelRevisionSelect+" WHERE tenant_id = ? AND template_id = ? ORDER BY revision ASC", tenant, source.Id); err != nil {
		_ = svcCtx.WhiteLabelTemplateModel.Delete(ctx, newID)
		return nil, status.Errorf(codes.Internal, "read source template revisions failed: %v", err)
	}
	createdRevisionIDs := make([]int64, 0, len(revisions))
	rollback := func() {
		for index := len(createdRevisionIDs) - 1; index >= 0; index-- {
			_ = svcCtx.WhiteLabelRevisionModel.Delete(ctx, createdRevisionIDs[index])
		}
		_ = svcCtx.WhiteLabelTemplateModel.Delete(ctx, newID)
	}
	for index := range revisions {
		revision := &revisions[index]
		inserted, insertErr := svcCtx.WhiteLabelRevisionModel.Insert(ctx, &models.TWhiteLabelTemplateRevision{
			TenantId: tenant, TemplateId: newID, Revision: revision.Revision,
			PackageNameRule: revision.PackageNameRule, ManifestPatch: revision.ManifestPatch,
			ResourcePatch: revision.ResourcePatch, ExtensionFiles: revision.ExtensionFiles,
			ExpectedArtifacts: revision.ExpectedArtifacts, Checksum: revision.Checksum,
			Status: int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_DRAFT), CreateBy: actorID(ctx),
		})
		if insertErr != nil {
			rollback()
			return nil, status.Errorf(codes.Internal, "copy template revision failed: %v", insertErr)
		}
		createdID, _ := inserted.LastInsertId()
		createdRevisionIDs = append(createdRevisionIDs, createdID)
	}
	item, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, newID)
	if err != nil {
		return nil, notFoundOrInternal(err, "copied white-label template")
	}
	return &core.WhiteLabelTemplateResp{Base: okBase(), Data: mapWhiteLabelTemplate(item)}, nil
}

func deleteWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelTemplateIdReq) (*core.RespBase, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	item, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template not found")
	}
	if item.Status != int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DRAFT) || item.PublishedRevision != 0 {
		return nil, status.Error(codes.FailedPrecondition, "only an unpublished draft template can be deleted")
	}
	var productCount int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &productCount, "SELECT COUNT(1) FROM t_white_label_product WHERE tenant_id = ? AND template_id = ?", tenant, item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "check template products failed: %v", err)
	}
	if productCount > 0 {
		return nil, status.Error(codes.FailedPrecondition, "template is referenced by white-label products")
	}
	var revisions []models.TWhiteLabelTemplateRevision
	if err := svcCtx.DB.QueryRowsCtx(ctx, &revisions, whiteLabelRevisionSelect+" WHERE tenant_id = ? AND template_id = ? ORDER BY id DESC", tenant, item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "read template revisions failed: %v", err)
	}
	for index := range revisions {
		if revisions[index].Status != int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_DRAFT) {
			return nil, status.Error(codes.FailedPrecondition, "template contains a non-draft revision")
		}
		if err := svcCtx.WhiteLabelRevisionModel.Delete(ctx, revisions[index].Id); err != nil {
			return nil, status.Errorf(codes.Internal, "delete template revision failed: %v", err)
		}
	}
	if err := svcCtx.WhiteLabelTemplateModel.Delete(ctx, item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete white-label template failed: %v", err)
	}
	return &core.RespBase{Base: okBase()}, nil
}

func listWhiteLabelTemplates(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelTemplateListReq) (*core.WhiteLabelTemplateListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &core.WhiteLabelTemplateListReq{}
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	if in.AppId > 0 {
		where = append(where, "app_id = ?")
		args = append(args, in.AppId)
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		where = append(where, "(template_code LIKE ? OR template_name LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if in.Status != core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(in.Status))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_white_label_template WHERE "+whereSQL, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list white-label templates count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TWhiteLabelTemplate
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, whiteLabelTemplateSelect+" WHERE "+whereSQL+" AND id > ? ORDER BY id ASC LIMIT ?", queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list white-label templates failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.WhiteLabelTemplate, 0, len(items))
	var next int64
	for index := range items {
		data = append(data, mapWhiteLabelTemplate(&items[index]))
		next = items[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.WhiteLabelTemplateListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func createWhiteLabelTemplateRevision(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateWhiteLabelTemplateRevisionReq) (*core.WhiteLabelTemplateRevisionResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.TemplateId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "template_id must be greater than zero")
	}
	template, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.TemplateId)
	if err != nil || template.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template not found")
	}
	if template.Status == int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DISABLED) {
		return nil, status.Error(codes.FailedPrecondition, "disabled template cannot create revisions")
	}
	documents, checksum, err := validateRevisionDocuments(in)
	if err != nil {
		return nil, err
	}
	if err := validateRevisionParameterReferences(stringValue(template.ParameterSchema), documents); err != nil {
		return nil, err
	}
	if existing, findErr := svcCtx.WhiteLabelRevisionModel.FindOneByTemplateIdChecksum(ctx, template.Id, checksum); findErr == nil && existing != nil {
		return nil, status.Error(codes.AlreadyExists, "an identical template revision already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check template revision failed: %v", findErr)
	}
	bindings, err := extractTemplateFileBindings(documents["resourcePatch"], documents["extensionFiles"])
	if err != nil {
		return nil, err
	}
	if err := bindTemplateFiles(ctx, svcCtx, template, bindings); err != nil {
		return nil, err
	}
	var nextRevision int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &nextRevision, "SELECT COALESCE(MAX(revision), 0) + 1 FROM t_white_label_template_revision WHERE template_id = ?", template.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "allocate template revision failed: %v", err)
	}
	result, err := svcCtx.WhiteLabelRevisionModel.Insert(ctx, &models.TWhiteLabelTemplateRevision{
		TenantId: tenant, TemplateId: template.Id, Revision: nextRevision,
		PackageNameRule: documents["packageNameRule"], ManifestPatch: nullString(documents["manifestPatch"]),
		ResourcePatch: nullString(documents["resourcePatch"]), ExtensionFiles: nullString(documents["extensionFiles"]),
		ExpectedArtifacts: nullString(documents["expectedArtifacts"]), Checksum: checksum,
		Status: int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_DRAFT), CreateBy: actorID(ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create template revision failed: %v", err)
	}
	id, _ := result.LastInsertId()
	item, err := svcCtx.WhiteLabelRevisionModel.FindOne(ctx, id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label template revision")
	}
	return &core.WhiteLabelTemplateRevisionResp{Base: okBase(), Data: mapWhiteLabelRevision(item)}, nil
}

func getWhiteLabelTemplateRevision(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelTemplateRevisionIdReq) (*core.WhiteLabelTemplateRevisionResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.TemplateId <= 0 || in.Revision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "template id and revision are required")
	}
	item, err := svcCtx.WhiteLabelRevisionModel.FindOneByTemplateIdRevision(ctx, in.TemplateId, int64(in.Revision))
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template revision not found")
	}
	return &core.WhiteLabelTemplateRevisionResp{Base: okBase(), Data: mapWhiteLabelRevision(item)}, nil
}

func updateWhiteLabelTemplateRevision(ctx context.Context, svcCtx *svc.ServiceContext, in *core.UpdateWhiteLabelTemplateRevisionReq) (*core.WhiteLabelTemplateRevisionResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.TemplateId <= 0 || in.Revision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "template id and revision are required")
	}
	template, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.TemplateId)
	if err != nil || template.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template not found")
	}
	if template.Status == int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DISABLED) {
		return nil, status.Error(codes.FailedPrecondition, "disabled template revision cannot be updated")
	}
	item, err := svcCtx.WhiteLabelRevisionModel.FindOneByTemplateIdRevision(ctx, in.TemplateId, int64(in.Revision))
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template revision not found")
	}
	if item.Status != int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_DRAFT) {
		return nil, status.Error(codes.FailedPrecondition, "published or superseded template revision is immutable")
	}
	documents, checksum, err := validateRevisionDocuments(&core.CreateWhiteLabelTemplateRevisionReq{
		TemplateId: in.TemplateId, PackageNameRuleJson: in.PackageNameRuleJson,
		ManifestPatchJson: in.ManifestPatchJson, ResourcePatchJson: in.ResourcePatchJson,
		ExtensionFilesJson: in.ExtensionFilesJson, ExpectedArtifactsJson: in.ExpectedArtifactsJson,
	})
	if err != nil {
		return nil, err
	}
	if err := validateRevisionParameterReferences(stringValue(template.ParameterSchema), documents); err != nil {
		return nil, err
	}
	bindings, err := extractTemplateFileBindings(documents["resourcePatch"], documents["extensionFiles"])
	if err != nil {
		return nil, err
	}
	if err := bindTemplateFiles(ctx, svcCtx, template, bindings); err != nil {
		return nil, err
	}
	if existing, findErr := svcCtx.WhiteLabelRevisionModel.FindOneByTemplateIdChecksum(ctx, template.Id, checksum); findErr == nil && existing.Id != item.Id {
		return nil, status.Error(codes.AlreadyExists, "an identical template revision already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check template revision failed: %v", findErr)
	}
	item.PackageNameRule = documents["packageNameRule"]
	item.ManifestPatch = nullString(documents["manifestPatch"])
	item.ResourcePatch = nullString(documents["resourcePatch"])
	item.ExtensionFiles = nullString(documents["extensionFiles"])
	item.ExpectedArtifacts = nullString(documents["expectedArtifacts"])
	item.Checksum = checksum
	if err := svcCtx.WhiteLabelRevisionModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update template revision failed: %v", err)
	}
	item, err = svcCtx.WhiteLabelRevisionModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label template revision")
	}
	return &core.WhiteLabelTemplateRevisionResp{Base: okBase(), Data: mapWhiteLabelRevision(item)}, nil
}

func deleteWhiteLabelTemplateRevision(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelTemplateRevisionIdReq) (*core.RespBase, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.TemplateId <= 0 || in.Revision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "template id and revision are required")
	}
	item, err := svcCtx.WhiteLabelRevisionModel.FindOneByTemplateIdRevision(ctx, in.TemplateId, int64(in.Revision))
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template revision not found")
	}
	if item.Status != int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_DRAFT) {
		return nil, status.Error(codes.FailedPrecondition, "only a draft template revision can be deleted")
	}
	var productCount int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &productCount, `SELECT COUNT(1) FROM t_white_label_product
WHERE tenant_id = ? AND template_id = ? AND template_revision = ?`, tenant, in.TemplateId, in.Revision); err != nil {
		return nil, status.Errorf(codes.Internal, "check template revision products failed: %v", err)
	}
	if productCount > 0 {
		return nil, status.Error(codes.FailedPrecondition, "template revision is referenced by white-label products")
	}
	if err := svcCtx.WhiteLabelRevisionModel.Delete(ctx, item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete template revision failed: %v", err)
	}
	return &core.RespBase{Base: okBase()}, nil
}

func listWhiteLabelTemplateRevisions(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelTemplateRevisionListReq) (*core.WhiteLabelTemplateRevisionListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.TemplateId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "template_id must be greater than zero")
	}
	template, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.TemplateId)
	if err != nil || template.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template not found")
	}
	cursor, limit := pageValues(in.Page)
	where := "tenant_id = ? AND template_id = ?"
	args := []any{tenant, in.TemplateId}
	if in.Status != core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_UNKNOWN {
		where += " AND status = ?"
		args = append(args, int64(in.Status))
	}
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_white_label_template_revision WHERE "+where, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list template revisions count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TWhiteLabelTemplateRevision
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, whiteLabelRevisionSelect+" WHERE "+where+" AND id > ? ORDER BY id ASC LIMIT ?", queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list template revisions failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.WhiteLabelTemplateRevision, 0, len(items))
	var next int64
	for index := range items {
		data = append(data, mapWhiteLabelRevision(&items[index]))
		next = items[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.WhiteLabelTemplateRevisionListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func publishWhiteLabelTemplate(ctx context.Context, svcCtx *svc.ServiceContext, in *core.PublishWhiteLabelTemplateReq) (*core.WhiteLabelTemplateResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 || in.Revision <= 0 {
		return nil, status.Error(codes.InvalidArgument, "template id and revision are required")
	}
	template, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.Id)
	if err != nil || template.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template not found")
	}
	if template.Status == int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DISABLED) {
		return nil, status.Error(codes.FailedPrecondition, "disabled template cannot be published")
	}
	revision, err := svcCtx.WhiteLabelRevisionModel.FindOneByTemplateIdRevision(ctx, template.Id, int64(in.Revision))
	if err != nil || revision.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "template revision not found")
	}
	if revision.Status != int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_DRAFT) {
		return nil, status.Error(codes.FailedPrecondition, "only draft revision can be published")
	}
	if err := svcCtx.WhiteLabelTemplateModel.PublishRevision(
		ctx, tenant, template.Id, int64(in.Revision),
		int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_DRAFT),
		int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_PUBLISHED),
		int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_SUPERSEDED),
		int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DISABLED),
	); err != nil {
		if errors.Is(err, models.ErrWhiteLabelTemplatePublishConflict) {
			return nil, status.Error(codes.FailedPrecondition, "template publish state changed; refresh and retry")
		}
		return nil, status.Errorf(codes.Internal, "publish template revision failed: %v", err)
	}
	template, err = svcCtx.WhiteLabelTemplateModel.FindOne(ctx, template.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label template")
	}
	return &core.WhiteLabelTemplateResp{Base: okBase(), Data: mapWhiteLabelTemplate(template)}, nil
}

func loadWhiteLabelDependencies(ctx context.Context, svcCtx *svc.ServiceContext, tenant, appID, templateID int64, revision int32, brandingID, signingID int64) (*whiteLabelDependency, error) {
	template, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, templateID)
	if err != nil || template.TenantId != tenant || template.AppId != appID {
		return nil, status.Error(codes.InvalidArgument, "white-label template is invalid")
	}
	revisionItem, err := svcCtx.WhiteLabelRevisionModel.FindOneByTemplateIdRevision(ctx, templateID, int64(revision))
	if err != nil || revisionItem.TenantId != tenant {
		return nil, status.Error(codes.InvalidArgument, "template revision is invalid")
	}
	branding, err := svcCtx.BrandingProfileModel.FindOne(ctx, brandingID)
	if err != nil || branding.TenantId != tenant || branding.AppId != appID {
		return nil, status.Error(codes.InvalidArgument, "branding profile is invalid")
	}
	signing, err := svcCtx.SigningConfigModel.FindOne(ctx, signingID)
	if err != nil || signing.TenantId != tenant || signing.AppId != appID {
		return nil, status.Error(codes.InvalidArgument, "signing configuration is invalid")
	}
	if len(stringValue(signing.CertificateSha256)) != 64 {
		return nil, status.Error(codes.FailedPrecondition, "signing configuration has no verified certificate fingerprint")
	}
	return &whiteLabelDependency{Template: template, Revision: revisionItem, Branding: branding, Signing: signing}, nil
}

func ensurePackageCertificate(ctx context.Context, svcCtx *svc.ServiceContext, tenant int64, packageName string, signing *models.TAppSigningConfig) error {
	fingerprint := strings.ToLower(strings.TrimSpace(stringValue(signing.CertificateSha256)))
	binding, err := svcCtx.PackageCertificateModel.FindOneByTenantIdPackageName(ctx, tenant, packageName)
	if err == nil {
		if binding.Status != 1 || !strings.EqualFold(binding.CertificateSha256, fingerprint) {
			return status.Error(codes.FailedPrecondition, "package_name is already bound to a different signing certificate")
		}
		return nil
	}
	if err != models.ErrNotFound {
		return status.Errorf(codes.Internal, "query package certificate binding failed: %v", err)
	}
	_, err = svcCtx.PackageCertificateModel.Insert(ctx, &models.TPackageCertificateBinding{
		TenantId: tenant, PackageName: packageName, CertificateSha256: fingerprint,
		SigningConfigId: signing.Id, Status: 1, CreateBy: actorID(ctx),
	})
	if err != nil {
		return status.Errorf(codes.Internal, "bind package signing certificate failed: %v", err)
	}
	return nil
}

func recordPackageCertificateBuildTask(ctx context.Context, svcCtx *svc.ServiceContext, tenant int64, packageName string, signing *models.TAppSigningConfig, taskID int64) error {
	binding, err := svcCtx.PackageCertificateModel.FindOneByTenantIdPackageName(ctx, tenant, packageName)
	if err != nil {
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
	if err := svcCtx.PackageCertificateModel.Update(ctx, binding); err != nil {
		return status.Errorf(codes.Internal, "record package certificate build history failed: %v", err)
	}
	return nil
}

func validateProductInput(ctx context.Context, svcCtx *svc.ServiceContext, tenant, appID, templateID int64, revision int32, brandingID, signingID int64, packageName, values string) (*whiteLabelDependency, string, string, error) {
	packageName, err := validatePackageName(packageName)
	if err != nil {
		return nil, "", "", err
	}
	dependency, err := loadWhiteLabelDependencies(ctx, svcCtx, tenant, appID, templateID, revision, brandingID, signingID)
	if err != nil {
		return nil, "", "", err
	}
	parameterValues, err := validateParameterValues(stringValue(dependency.Template.ParameterSchema), values)
	if err != nil {
		return nil, "", "", err
	}
	return dependency, packageName, parameterValues, nil
}

func createWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateWhiteLabelProductReq) (*core.WhiteLabelProductResp, error) {
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
	code := strings.TrimSpace(in.ProductCode)
	if !codePattern.MatchString(code) {
		return nil, status.Error(codes.InvalidArgument, "product_code must use 2-64 lowercase letters, digits, underscores or hyphens")
	}
	if err := requireText(in.ProductName, "product_name", 128); err != nil {
		return nil, err
	}
	app, err := svcCtx.ApplicationModel.FindOne(ctx, in.AppId)
	if err != nil || app.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "application not found")
	}
	dependency, packageName, values, err := validateProductInput(ctx, svcCtx, tenant, in.AppId, in.TemplateId, in.TemplateRevision, in.BrandingProfileId, in.SigningConfigId, in.PackageName, in.ParameterValuesJson)
	if err != nil {
		return nil, err
	}
	if _, findErr := svcCtx.WhiteLabelProductModel.FindOneByTenantIdProductCode(ctx, tenant, code); findErr == nil {
		return nil, status.Error(codes.AlreadyExists, "product_code already exists")
	} else if findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check product_code failed: %v", findErr)
	}
	if _, findErr := svcCtx.WhiteLabelProductModel.FindOneByTenantIdPackageName(ctx, tenant, packageName); findErr == nil {
		return nil, status.Error(codes.AlreadyExists, "package_name already exists")
	} else if findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check package_name failed: %v", findErr)
	}
	if err := ensurePackageCertificate(ctx, svcCtx, tenant, packageName, dependency.Signing); err != nil {
		return nil, err
	}
	result, err := svcCtx.WhiteLabelProductModel.Insert(ctx, &models.TWhiteLabelProduct{
		TenantId: tenant, AppId: in.AppId, ProductCode: code, ProductName: strings.TrimSpace(in.ProductName),
		TemplateId: in.TemplateId, TemplateRevision: int64(in.TemplateRevision), BrandingProfileId: in.BrandingProfileId,
		PackageName: packageName, SigningConfigId: in.SigningConfigId, ParameterValues: nullString(values),
		Status: int64(core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_DRAFT), CreateBy: actorID(ctx),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create white-label product failed: %v", err)
	}
	id, _ := result.LastInsertId()
	item, err := svcCtx.WhiteLabelProductModel.FindOne(ctx, id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label product")
	}
	return &core.WhiteLabelProductResp{Base: okBase(), Data: mapWhiteLabelProduct(item)}, nil
}

func updateWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, in *core.UpdateWhiteLabelProductReq) (*core.WhiteLabelProductResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	item, err := svcCtx.WhiteLabelProductModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label product not found")
	}
	if item.Status == int64(core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_ENABLED) {
		return nil, status.Error(codes.FailedPrecondition, "enabled product must be disabled before editing")
	}
	if err := requireText(in.ProductName, "product_name", 128); err != nil {
		return nil, err
	}
	templateID := in.TemplateId
	if templateID <= 0 {
		templateID = item.TemplateId
	}
	templateRevision := in.TemplateRevision
	if templateRevision <= 0 {
		templateRevision = int32(item.TemplateRevision)
	}
	dependency, packageName, values, err := validateProductInput(ctx, svcCtx, tenant, item.AppId, templateID, templateRevision, in.BrandingProfileId, in.SigningConfigId, in.PackageName, in.ParameterValuesJson)
	if err != nil {
		return nil, err
	}
	if existing, findErr := svcCtx.WhiteLabelProductModel.FindOneByTenantIdPackageName(ctx, tenant, packageName); findErr == nil && existing.Id != item.Id {
		return nil, status.Error(codes.AlreadyExists, "package_name already exists")
	} else if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check package_name failed: %v", findErr)
	}
	if err := ensurePackageCertificate(ctx, svcCtx, tenant, packageName, dependency.Signing); err != nil {
		return nil, err
	}
	item.ProductName = strings.TrimSpace(in.ProductName)
	item.TemplateId = templateID
	item.TemplateRevision = int64(templateRevision)
	item.BrandingProfileId = in.BrandingProfileId
	item.PackageName = packageName
	item.SigningConfigId = in.SigningConfigId
	item.ParameterValues = nullString(values)
	if err := svcCtx.WhiteLabelProductModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "update white-label product failed: %v", err)
	}
	item, err = svcCtx.WhiteLabelProductModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label product")
	}
	return &core.WhiteLabelProductResp{Base: okBase(), Data: mapWhiteLabelProduct(item)}, nil
}

func deleteWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelProductIdReq) (*core.RespBase, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	item, err := svcCtx.WhiteLabelProductModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label product not found")
	}
	if item.Status == int64(core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_ENABLED) {
		return nil, status.Error(codes.FailedPrecondition, "enabled white-label product cannot be deleted")
	}
	var buildCount int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &buildCount, "SELECT COUNT(1) FROM t_build_task WHERE tenant_id = ? AND white_label_product_id = ?", tenant, item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "check white-label product builds failed: %v", err)
	}
	if buildCount > 0 {
		return nil, status.Error(codes.FailedPrecondition, "white-label product has build history and cannot be deleted")
	}
	if err := svcCtx.WhiteLabelProductModel.Delete(ctx, item.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete white-label product failed: %v", err)
	}
	return &core.RespBase{Base: okBase()}, nil
}

func changeWhiteLabelTemplateStatus(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ChangeWhiteLabelTemplateStatusReq) (*core.WhiteLabelTemplateResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	if in.Status != core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_PUBLISHED &&
		in.Status != core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_DISABLED {
		return nil, status.Error(codes.InvalidArgument, "template status can only be published or disabled")
	}
	item, err := svcCtx.WhiteLabelTemplateModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label template not found")
	}
	if in.Status == core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_PUBLISHED && item.PublishedRevision <= 0 {
		return nil, status.Error(codes.FailedPrecondition, "template has no published revision")
	}
	item.Status = int64(in.Status)
	if err := svcCtx.WhiteLabelTemplateModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "change white-label template status failed: %v", err)
	}
	item, err = svcCtx.WhiteLabelTemplateModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label template")
	}
	return &core.WhiteLabelTemplateResp{Base: okBase(), Data: mapWhiteLabelTemplate(item)}, nil
}

func getWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelProductIdReq) (*core.WhiteLabelProductResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	item, err := svcCtx.WhiteLabelProductModel.FindOne(ctx, in.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label product")
	}
	if err := ensureTenant(item.TenantId, tenant); err != nil {
		return nil, err
	}
	return &core.WhiteLabelProductResp{Base: okBase(), Data: mapWhiteLabelProduct(item)}, nil
}

func listWhiteLabelProducts(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelProductListReq) (*core.WhiteLabelProductListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil {
		in = &core.WhiteLabelProductListReq{}
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"tenant_id = ?"}
	args := []any{tenant}
	if in.AppId > 0 {
		where = append(where, "app_id = ?")
		args = append(args, in.AppId)
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		where = append(where, "(product_code LIKE ? OR product_name LIKE ? OR package_name LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if in.Status != core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(in.Status))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_white_label_product WHERE "+whereSQL, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "list white-label products count failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var items []models.TWhiteLabelProduct
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, whiteLabelProductSelect+" WHERE "+whereSQL+" AND id > ? ORDER BY id ASC LIMIT ?", queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list white-label products failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.WhiteLabelProduct, 0, len(items))
	var next int64
	for index := range items {
		data = append(data, mapWhiteLabelProduct(&items[index]))
		next = items[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.WhiteLabelProductListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func preflightWhiteLabelProduct(ctx context.Context, svcCtx *svc.ServiceContext, in *core.WhiteLabelProductIdReq) (*core.WhiteLabelProductPreflightResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	product, err := svcCtx.WhiteLabelProductModel.FindOne(ctx, in.Id)
	if err != nil || product.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label product not found")
	}
	dependency, err := loadWhiteLabelDependencies(ctx, svcCtx, tenant, product.AppId, product.TemplateId, int32(product.TemplateRevision), product.BrandingProfileId, product.SigningConfigId)
	report := whiteLabelPreflightReport{Compatible: true}
	add := func(name string, passed bool, message string) {
		report.Checks = append(report.Checks, whiteLabelPreflightCheck{Name: name, Passed: passed, Message: message})
		if !passed {
			report.Compatible = false
		}
	}
	if err != nil {
		add("dependencies", false, err.Error())
	} else {
		add("template_published", dependency.Template.Status == int64(core.WhiteLabelTemplateStatus_WHITE_LABEL_TEMPLATE_STATUS_PUBLISHED) && dependency.Template.PublishedRevision == product.TemplateRevision, "模板修订必须处于当前发布状态")
		add("revision_immutable", dependency.Revision.Status == int64(core.WhiteLabelRevisionStatus_WHITE_LABEL_REVISION_STATUS_PUBLISHED), "模板修订必须已发布")
		add("branding_enabled", dependency.Branding.Status == int64(core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_ENABLED), "品牌配置必须启用")
		add("signing_enabled", dependency.Signing.Status == 1 && len(stringValue(dependency.Signing.CertificateSha256)) == 64, "签名配置必须启用且证书已验证")
		if _, valuesErr := validateParameterValues(stringValue(dependency.Template.ParameterSchema), stringValue(product.ParameterValues)); valuesErr != nil {
			add("parameters", false, valuesErr.Error())
		} else {
			add("parameters", true, "模板参数符合Schema")
		}
		if bindErr := ensurePackageCertificate(ctx, svcCtx, tenant, product.PackageName, dependency.Signing); bindErr != nil {
			add("certificate_binding", false, bindErr.Error())
		} else {
			add("certificate_binding", true, "包名证书绑定一致")
		}
		var compatibleCount int64
		queryErr := svcCtx.DB.QueryRowCtx(ctx, &compatibleCount, `SELECT COUNT(1) FROM t_branding_preflight
WHERE tenant_id = ? AND branding_profile_id = ? AND branding_revision = ? AND version_id = ? AND status = ?`,
			tenant, product.BrandingProfileId, dependency.Branding.Revision, dependency.Template.SourceVersionId,
			int64(core.BrandingPreflightStatus_BRANDING_PREFLIGHT_STATUS_COMPATIBLE))
		add("branding_preflight", queryErr == nil && compatibleCount > 0, "源APK与品牌修订必须通过V2兼容性预检")
	}
	reportJSON, _ := json.Marshal(report)
	return &core.WhiteLabelProductPreflightResp{Base: okBase(), Compatible: report.Compatible, ReportJson: string(reportJSON)}, nil
}

func changeWhiteLabelProductStatus(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ChangeWhiteLabelProductStatusReq) (*core.WhiteLabelProductResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	if in.Status != core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_DRAFT && in.Status != core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_ENABLED && in.Status != core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_DISABLED {
		return nil, status.Error(codes.InvalidArgument, "invalid white-label product status")
	}
	item, err := svcCtx.WhiteLabelProductModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "white-label product not found")
	}
	if in.Status == core.WhiteLabelProductStatus_WHITE_LABEL_PRODUCT_STATUS_ENABLED {
		preflight, preflightErr := preflightWhiteLabelProduct(ctx, svcCtx, &core.WhiteLabelProductIdReq{Id: item.Id})
		if preflightErr != nil {
			return nil, preflightErr
		}
		if !preflight.Compatible {
			return nil, status.Errorf(codes.FailedPrecondition, "white-label product preflight failed: %s", preflight.ReportJson)
		}
	}
	item.Status = int64(in.Status)
	if err := svcCtx.WhiteLabelProductModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "change white-label product status failed: %v", err)
	}
	item, err = svcCtx.WhiteLabelProductModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, notFoundOrInternal(err, "white-label product")
	}
	return &core.WhiteLabelProductResp{Base: okBase(), Data: mapWhiteLabelProduct(item)}, nil
}
