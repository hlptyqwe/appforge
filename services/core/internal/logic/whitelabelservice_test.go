package logic

import (
	"strings"
	"testing"

	"appforge/proto/core"
)

func TestValidatePackageName(t *testing.T) {
	if value, err := validatePackageName("com.customer.product_1"); err != nil || value != "com.customer.product_1" {
		t.Fatalf("expected valid package name, value=%q err=%v", value, err)
	}
	for _, value := range []string{"Com.Customer.App", "com", "com.customer.bad-name", "com..app"} {
		if _, err := validatePackageName(value); err == nil {
			t.Fatalf("expected package name %q to be rejected", value)
		}
	}
}

func TestValidateRevisionDocumentsRejectsExecutableAndUnsafePath(t *testing.T) {
	base := &core.CreateWhiteLabelTemplateRevisionReq{
		PackageNameRuleJson: `{}`,
		ManifestPatchJson:   `[{"op":"manifest.setPackage"}]`,
		ResourcePatchJson:   `[]`,
		ExtensionFilesJson:  `[]`,
	}
	if _, checksum, err := validateRevisionDocuments(base); err != nil || len(checksum) != 64 {
		t.Fatalf("expected valid revision, checksum=%q err=%v", checksum, err)
	}

	base.ExtensionFilesJson = `[{"op":"extension.writeValidatedFile","path":"../secret.json","content":{}}]`
	if _, _, err := validateRevisionDocuments(base); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected unsafe path rejection, err=%v", err)
	}

	base.ExtensionFilesJson = `[{"op":"extension.writeValidatedFile","path":"assets/config.json","content":{},"command":"sh -c evil"}]`
	if _, _, err := validateRevisionDocuments(base); err == nil || !strings.Contains(err.Error(), "command") {
		t.Fatalf("expected command rejection, err=%v", err)
	}

	base.ExtensionFilesJson = `[]`
	base.ResourcePatchJson = `[{"op":"resource.replaceFile","path":"res/raw/customer.json"}]`
	if _, _, err := validateRevisionDocuments(base); err == nil || !strings.Contains(err.Error(), "objectId is required") {
		t.Fatalf("expected missing template file object rejection, err=%v", err)
	}

	base.ResourcePatchJson = `[{"op":"resource.replaceFile","path":"res/raw/customer.json","objectId":17}]`
	documents, _, err := validateRevisionDocuments(base)
	if err != nil {
		t.Fatalf("expected bound template file operation to be valid: %v", err)
	}
	bindings, err := extractTemplateFileBindings(documents["resourcePatch"], documents["extensionFiles"])
	if err != nil || len(bindings) != 1 || bindings[0].ObjectID != 17 {
		t.Fatalf("unexpected template file bindings: %#v err=%v", bindings, err)
	}
}

func TestValidateParameterValuesEnforcesSchema(t *testing.T) {
	schema := `{"type":"object","properties":{"oauthScheme":{"type":"string"},"enabled":{"type":"boolean"}},"required":["oauthScheme"],"additionalProperties":false}`
	if _, err := validateParameterSchema(schema); err != nil {
		t.Fatalf("expected schema with required parameters to be accepted: %v", err)
	}
	if normalized, err := validateParameterValues(schema, `{"oauthScheme":"demo","enabled":true}`); err != nil || normalized == "" {
		t.Fatalf("expected valid parameters, normalized=%q err=%v", normalized, err)
	}
	if _, err := validateParameterValues(schema, `{"enabled":true}`); err == nil {
		t.Fatal("expected missing required parameter to be rejected")
	}
	if _, err := validateParameterValues(schema, `{"oauthScheme":"demo","extra":"value"}`); err == nil {
		t.Fatal("expected undeclared parameter to be rejected")
	}
}

func TestSensitiveParameterSchemaAndEmbeddedSecretPolicy(t *testing.T) {
	schema := `{"type":"object","properties":{"clientSecret":{"type":"string","sensitive":true}},"required":["clientSecret"],"additionalProperties":false}`
	if _, err := validateParameterSchema(schema); err != nil {
		t.Fatalf("expected sensitive string schema to be accepted: %v", err)
	}
	if _, err := validateParameterSchema(`{"type":"object","properties":{"secret":{"type":"object","sensitive":true}},"additionalProperties":false}`); err == nil {
		t.Fatal("expected non-string sensitive parameter to be rejected")
	}
	request := &core.CreateWhiteLabelTemplateRevisionReq{
		PackageNameRuleJson: `{}`,
		ManifestPatchJson:   `[{"op":"manifest.setPackage"}]`,
		ResourcePatchJson:   `[]`,
		ExtensionFilesJson:  `[{"op":"extension.writeValidatedFile","path":"assets/private.json","content":{"client_secret":"plaintext"}}]`,
	}
	if _, _, err := validateRevisionDocuments(request); err == nil || !strings.Contains(err.Error(), "plaintext sensitive field") {
		t.Fatalf("expected inline plaintext secret to be rejected, err=%v", err)
	}
}

func TestRevisionOperationSchemaAndParameterReferences(t *testing.T) {
	request := &core.CreateWhiteLabelTemplateRevisionReq{
		PackageNameRuleJson: `{}`,
		ManifestPatchJson:   `[{"op":"manifest.replaceIntentScheme","old":"source","new":"{{parameters.oauthScheme}}"}]`,
		ResourcePatchJson:   `[{"op":"resource.replaceString","name":"oauth_scheme","valueParameter":"oauthScheme"}]`,
		ExtensionFilesJson:  `[]`,
	}
	documents, _, err := validateRevisionDocuments(request)
	if err != nil {
		t.Fatalf("expected valid operation schemas: %v", err)
	}
	schema := `{"type":"object","properties":{"oauthScheme":{"type":"string"}},"additionalProperties":false}`
	if err := validateRevisionParameterReferences(schema, documents); err != nil {
		t.Fatalf("expected declared parameter references: %v", err)
	}
	if err := validateRevisionParameterReferences(`{"type":"object","properties":{},"additionalProperties":false}`, documents); err == nil {
		t.Fatal("expected undeclared parameter reference to be rejected")
	}
	request.ManifestPatchJson = `[{"op":"manifest.setPackage","script":"evil"}]`
	if _, _, err := validateRevisionDocuments(request); err == nil {
		t.Fatal("expected unsupported operation field to be rejected")
	}
}
