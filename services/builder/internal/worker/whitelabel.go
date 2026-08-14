package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	whiteLabelAssetPath                = "assets/appforge/template.json"
	manifestComponentPlaceholderPrefix = "__APPFORGE_COMPONENT_"
)

var (
	androidPackagePattern             = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	manifestComponentTagPattern       = regexp.MustCompile(`(?s)<(?:application|activity|activity-alias|service|receiver|provider|instrumentation)\b[^>]*>`)
	manifestComponentAttributePattern = regexp.MustCompile(`(android:(?:name|targetActivity)\s*=\s*")([^"]+)(")`)
)

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
	parameters            map[string]any
}

type whiteLabelPublicAsset struct {
	SchemaVersion     int    `json:"schemaVersion"`
	ProductID         int64  `json:"productId"`
	ProductCode       string `json:"productCode"`
	TemplateID        int64  `json:"templateId"`
	TemplateCode      string `json:"templateCode"`
	TemplateRevision  int64  `json:"templateRevision"`
	TemplateChecksum  string `json:"templateChecksum"`
	TargetPackageName string `json:"targetPackageName"`
}

type templateOperation struct {
	Op               string          `json:"op"`
	Target           string          `json:"target"`
	Name             string          `json:"name"`
	Value            json.RawMessage `json:"value"`
	ValueParameter   string          `json:"valueParameter"`
	Old              string          `json:"old"`
	New              string          `json:"new"`
	Path             string          `json:"path"`
	Content          json.RawMessage `json:"content"`
	ContentParameter string          `json:"contentParameter"`
	ObjectID         int64           `json:"objectId"`
}

func decodeWhiteLabelBuildSnapshot(value string) (*whiteLabelBuildSnapshot, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var snapshot whiteLabelBuildSnapshot
	if err := json.Unmarshal([]byte(value), &snapshot); err != nil {
		return nil, fmt.Errorf("decode white-label snapshot: %w", err)
	}
	if snapshot.ProductID <= 0 || snapshot.TemplateID <= 0 || snapshot.TemplateRevision <= 0 ||
		len(snapshot.TemplateChecksum) != 64 || len(snapshot.CertificateSHA256) != 64 ||
		!androidPackagePattern.MatchString(snapshot.OriginalPackageName) || !androidPackagePattern.MatchString(snapshot.TargetPackageName) {
		return nil, fmt.Errorf("white-label snapshot is incomplete")
	}
	if err := json.Unmarshal([]byte(snapshot.ParameterValuesJSON), &snapshot.parameters); err != nil {
		return nil, fmt.Errorf("decode white-label parameters: %w", err)
	}
	return &snapshot, nil
}

func (s *whiteLabelBuildSnapshot) decryptSensitiveParameters(open func(string) (string, error)) error {
	if s == nil {
		return nil
	}
	value, err := decryptSensitiveValue(s.parameters, open)
	if err != nil {
		return err
	}
	parameters, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("white-label parameters must be an object")
	}
	s.parameters = parameters
	return nil
}

func decryptSensitiveValue(value any, open func(string) (string, error)) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			decoded, err := decryptSensitiveValue(child, open)
			if err != nil {
				return nil, fmt.Errorf("decrypt parameter %s: %w", key, err)
			}
			typed[key] = decoded
		}
		return typed, nil
	case []any:
		for index, child := range typed {
			decoded, err := decryptSensitiveValue(child, open)
			if err != nil {
				return nil, err
			}
			typed[index] = decoded
		}
		return typed, nil
	case string:
		if strings.HasPrefix(strings.TrimSpace(typed), "sb1.") {
			decoded, err := open(typed)
			if err != nil {
				return nil, err
			}
			return decoded, nil
		}
	}
	return value, nil
}

func (s *whiteLabelBuildSnapshot) publicAsset() whiteLabelPublicAsset {
	return whiteLabelPublicAsset{
		SchemaVersion: 1, ProductID: s.ProductID, ProductCode: s.ProductCode,
		TemplateID: s.TemplateID, TemplateCode: s.TemplateCode,
		TemplateRevision: s.TemplateRevision, TemplateChecksum: s.TemplateChecksum,
		TargetPackageName: s.TargetPackageName,
	}
}

func applyWhiteLabelTemplate(decodedDir, manifestPath string, snapshot *whiteLabelBuildSnapshot, templateFiles map[int64]string) error {
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest for package rewrite: %w", err)
	}
	manifestText := string(manifest)
	manifestText, componentNames, err := protectManifestComponentNames(manifestText, snapshot.OriginalPackageName)
	if err != nil {
		return err
	}
	packagePattern := regexp.MustCompile(`\bpackage="` + regexp.QuoteMeta(snapshot.OriginalPackageName) + `"`)
	if !packagePattern.MatchString(manifestText) {
		return fmt.Errorf("source package %q was not found in decoded manifest", snapshot.OriginalPackageName)
	}
	manifestText = packagePattern.ReplaceAllString(manifestText, `package="`+snapshot.TargetPackageName+`"`)
	manifestText = strings.ReplaceAll(manifestText, snapshot.OriginalPackageName, snapshot.TargetPackageName)
	manifestText = restoreManifestComponentNames(manifestText, componentNames)
	if err := os.WriteFile(manifestPath, []byte(manifestText), 0o600); err != nil {
		return fmt.Errorf("write package-rewritten manifest: %w", err)
	}
	if err := replacePackageInResourceXML(decodedDir, snapshot.OriginalPackageName, snapshot.TargetPackageName); err != nil {
		return err
	}
	if err := applyManifestOperations(manifestPath, snapshot); err != nil {
		return err
	}
	if err := applyFileOperations(decodedDir, snapshot.ResourcePatchJSON, snapshot, templateFiles); err != nil {
		return err
	}
	if err := applyFileOperations(decodedDir, snapshot.ExtensionFilesJSON, snapshot, templateFiles); err != nil {
		return err
	}
	return verifyNoCriticalOldPackageReferences(decodedDir, snapshot.OriginalPackageName)
}

func protectManifestComponentNames(manifest, packageName string) (string, []string, error) {
	if strings.Contains(manifest, manifestComponentPlaceholderPrefix) {
		return "", nil, fmt.Errorf("source manifest contains reserved component placeholder")
	}
	componentNames := make([]string, 0, 8)
	protected := manifestComponentTagPattern.ReplaceAllStringFunc(manifest, func(tag string) string {
		return manifestComponentAttributePattern.ReplaceAllStringFunc(tag, func(attribute string) string {
			parts := manifestComponentAttributePattern.FindStringSubmatch(attribute)
			componentName := parts[2]
			switch {
			case strings.HasPrefix(componentName, "."):
				componentName = packageName + componentName
			case !strings.Contains(componentName, "."):
				componentName = packageName + "." + componentName
			}
			placeholder := fmt.Sprintf("%s%d__", manifestComponentPlaceholderPrefix, len(componentNames))
			componentNames = append(componentNames, componentName)
			return parts[1] + placeholder + parts[3]
		})
	})
	return protected, componentNames, nil
}

func restoreManifestComponentNames(manifest string, componentNames []string) string {
	for index, componentName := range componentNames {
		placeholder := fmt.Sprintf("%s%d__", manifestComponentPlaceholderPrefix, index)
		manifest = strings.ReplaceAll(manifest, placeholder, componentName)
	}
	return manifest
}

func replacePackageInResourceXML(decodedDir, oldPackage, newPackage string) error {
	files, err := filepath.Glob(filepath.Join(decodedDir, "res", "*", "*.xml"))
	if err != nil {
		return err
	}
	for _, filename := range files {
		content, readErr := os.ReadFile(filename)
		if readErr != nil {
			return readErr
		}
		if !strings.Contains(string(content), oldPackage) {
			continue
		}
		if err := os.WriteFile(filename, []byte(strings.ReplaceAll(string(content), oldPackage, newPackage)), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func applyManifestOperations(manifestPath string, snapshot *whiteLabelBuildSnapshot) error {
	operations, err := decodeTemplateOperations(snapshot.ManifestPatchJSON)
	if err != nil {
		return fmt.Errorf("decode manifest patch: %w", err)
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	manifest := string(content)
	for _, operation := range operations {
		switch operation.Op {
		case "manifest.setPackage":
			continue
		case "manifest.replaceProviderAuthority", "manifest.replaceIntentScheme", "manifest.replaceAppLinkHost":
			oldValue, err := snapshot.resolveString(operation.Old)
			if err != nil {
				return err
			}
			newValue, err := snapshot.resolveString(operation.New)
			if err != nil {
				return err
			}
			if oldValue == "" || !strings.Contains(manifest, oldValue) {
				return fmt.Errorf("manifest patch source %q was not found", oldValue)
			}
			manifest = strings.ReplaceAll(manifest, oldValue, newValue)
		case "manifest.setAttribute":
			value, err := snapshot.operationValue(operation)
			if err != nil {
				return err
			}
			if operation.Target != "application" || !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9:_-]*$`).MatchString(operation.Name) {
				return fmt.Errorf("manifest.setAttribute target or name is invalid")
			}
			escaped := new(strings.Builder)
			if err := xml.EscapeText(escaped, []byte(value)); err != nil {
				return err
			}
			applicationPattern := regexp.MustCompile(`(?s)<application\b[^>]*>`)
			applicationTag := applicationPattern.FindString(manifest)
			if applicationTag == "" {
				return fmt.Errorf("manifest application element was not found")
			}
			attributePattern := regexp.MustCompile(`\s+` + regexp.QuoteMeta(operation.Name) + `="[^"]*"`)
			if attributePattern.MatchString(applicationTag) {
				applicationTag = attributePattern.ReplaceAllString(applicationTag, ` `+operation.Name+`="`+escaped.String()+`"`)
			} else {
				applicationTag = strings.TrimSuffix(applicationTag, ">") + ` ` + operation.Name + `="` + escaped.String() + `">`
			}
			manifest = applicationPattern.ReplaceAllString(manifest, applicationTag)
		default:
			return fmt.Errorf("manifest operation %q is unsupported", operation.Op)
		}
	}
	return os.WriteFile(manifestPath, []byte(manifest), 0o600)
}

func applyFileOperations(decodedDir, document string, snapshot *whiteLabelBuildSnapshot, templateFiles map[int64]string) error {
	operations, err := decodeTemplateOperations(document)
	if err != nil {
		return err
	}
	for _, operation := range operations {
		switch operation.Op {
		case "resource.replaceString":
			if !regexp.MustCompile(`^[a-z0-9_.]+$`).MatchString(operation.Name) {
				return fmt.Errorf("resource string name is invalid")
			}
			value, err := snapshot.operationValue(operation)
			if err != nil {
				return err
			}
			if err := replaceStringResource(decodedDir, operation.Name, value); err != nil {
				return err
			}
		case "asset.writeJson", "extension.writeValidatedFile":
			if err := writeControlledExtension(decodedDir, operation, snapshot, templateFiles); err != nil {
				return err
			}
		case "resource.replaceFile":
			if err := replaceControlledResourceFile(decodedDir, operation, templateFiles); err != nil {
				return err
			}
		default:
			return fmt.Errorf("file operation %q is unsupported", operation.Op)
		}
	}
	return nil
}

func decodeTemplateOperations(document string) ([]templateOperation, error) {
	if strings.TrimSpace(document) == "" {
		return nil, nil
	}
	var operations []templateOperation
	if err := json.Unmarshal([]byte(document), &operations); err != nil {
		return nil, err
	}
	return operations, nil
}

func (s *whiteLabelBuildSnapshot) resolveString(value string) (string, error) {
	value = strings.ReplaceAll(value, "{{packageName}}", s.TargetPackageName)
	value = strings.ReplaceAll(value, "{{originalPackageName}}", s.OriginalPackageName)
	parameterPattern := regexp.MustCompile(`\{\{parameters\.([a-zA-Z0-9_.-]+)\}\}`)
	var resolveErr error
	value = parameterPattern.ReplaceAllStringFunc(value, func(token string) string {
		match := parameterPattern.FindStringSubmatch(token)
		parameter, exists := s.parameters[match[1]]
		if !exists {
			resolveErr = fmt.Errorf("template parameter %s is unavailable", match[1])
			return ""
		}
		text, ok := parameter.(string)
		if !ok {
			resolveErr = fmt.Errorf("template parameter %s must be a string", match[1])
			return ""
		}
		return text
	})
	return value, resolveErr
}

func (s *whiteLabelBuildSnapshot) operationValue(operation templateOperation) (string, error) {
	if operation.ValueParameter != "" {
		value, exists := s.parameters[operation.ValueParameter]
		if !exists {
			return "", fmt.Errorf("template parameter %s is unavailable", operation.ValueParameter)
		}
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("template parameter %s must be a string", operation.ValueParameter)
		}
		return text, nil
	}
	if len(operation.Value) == 0 {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(operation.Value, &text); err != nil {
		return "", fmt.Errorf("operation value must be a string")
	}
	return s.resolveString(text)
}

func replaceStringResource(decodedDir, name, value string) error {
	files, _ := filepath.Glob(filepath.Join(decodedDir, "res", "values*", "*.xml"))
	pattern := regexp.MustCompile(`(?s)(<string\s+[^>]*name="` + regexp.QuoteMeta(name) + `"[^>]*>).*?(</string>)`)
	escaped := new(strings.Builder)
	if err := xml.EscapeText(escaped, []byte(value)); err != nil {
		return err
	}
	replaced := 0
	for _, filename := range files {
		content, err := os.ReadFile(filename)
		if err != nil || !pattern.Match(content) {
			continue
		}
		updated := pattern.ReplaceAllString(string(content), `${1}`+escaped.String()+`${2}`)
		if err := os.WriteFile(filename, []byte(updated), 0o600); err != nil {
			return err
		}
		replaced++
	}
	if replaced == 0 {
		return fmt.Errorf("string resource %q was not found", name)
	}
	return nil
}

func writeControlledExtension(decodedDir string, operation templateOperation, snapshot *whiteLabelBuildSnapshot, templateFiles map[int64]string) error {
	target := filepath.ToSlash(filepath.Clean(operation.Path))
	if target == "." || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "../") || strings.Contains(target, "/../") ||
		(!strings.HasPrefix(target, "assets/") && !strings.HasPrefix(target, "res/raw/")) {
		return fmt.Errorf("extension target path is not allowed")
	}
	extension := strings.ToLower(filepath.Ext(target))
	if extension != ".json" && extension != ".xml" && extension != ".txt" {
		return fmt.Errorf("extension file type is not allowed")
	}
	filename := filepath.Join(decodedDir, filepath.FromSlash(target))
	if operation.ObjectID > 0 {
		source, ok := templateFiles[operation.ObjectID]
		if !ok {
			return fmt.Errorf("template file object %d is unavailable", operation.ObjectID)
		}
		encoded, err := readControlledTemplateFile(source, target, 2*1024*1024)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
			return err
		}
		return os.WriteFile(filename, encoded, 0o600)
	}
	var value any
	if operation.ContentParameter != "" {
		var exists bool
		value, exists = snapshot.parameters[operation.ContentParameter]
		if !exists {
			return fmt.Errorf("template parameter %s is unavailable", operation.ContentParameter)
		}
	} else if len(operation.Content) > 0 {
		if err := json.Unmarshal(operation.Content, &value); err != nil {
			return fmt.Errorf("extension content is invalid")
		}
	} else if len(operation.Value) > 0 {
		if err := json.Unmarshal(operation.Value, &value); err != nil {
			return fmt.Errorf("extension value is invalid")
		}
	} else {
		return fmt.Errorf("extension content is required")
	}
	var encoded []byte
	if extension == ".json" {
		var err error
		if text, ok := value.(string); ok {
			if !json.Valid([]byte(text)) {
				return fmt.Errorf("extension JSON content is invalid")
			}
			encoded = []byte(text)
		} else {
			encoded, err = json.Marshal(value)
			if err != nil {
				return err
			}
		}
	} else {
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("extension XML/TXT content must be a string")
		}
		encoded = []byte(text)
	}
	if len(encoded) > 256*1024 {
		return fmt.Errorf("extension content exceeds 256 KiB")
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filename, encoded, 0o600)
}

func replaceControlledResourceFile(decodedDir string, operation templateOperation, templateFiles map[int64]string) error {
	target := filepath.ToSlash(filepath.Clean(operation.Path))
	if operation.ObjectID <= 0 || target == "." || strings.HasPrefix(target, "/") || strings.HasPrefix(target, "../") ||
		strings.Contains(target, "/../") || !strings.HasPrefix(target, "res/") {
		return fmt.Errorf("resource replacement target or object binding is invalid")
	}
	extension := strings.ToLower(filepath.Ext(target))
	if extension != ".json" && extension != ".xml" && extension != ".txt" && extension != ".png" && extension != ".webp" {
		return fmt.Errorf("resource replacement file type is not allowed")
	}
	filename := filepath.Join(decodedDir, filepath.FromSlash(target))
	info, err := os.Stat(filename)
	if err != nil || info.IsDir() {
		return fmt.Errorf("resource replacement target %q does not exist", target)
	}
	source, ok := templateFiles[operation.ObjectID]
	if !ok {
		return fmt.Errorf("template file object %d is unavailable", operation.ObjectID)
	}
	encoded, err := readControlledTemplateFile(source, target, 2*1024*1024)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, encoded, 0o600)
}

func readControlledTemplateFile(filename, target string, maximum int64) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("template file exceeds %d bytes", maximum)
	}
	switch strings.ToLower(filepath.Ext(target)) {
	case ".json":
		if !json.Valid(data) {
			return nil, fmt.Errorf("template JSON file is invalid")
		}
	case ".xml":
		decoder := xml.NewDecoder(bytes.NewReader(data))
		for {
			token, decodeErr := decoder.Token()
			if decodeErr == io.EOF {
				break
			}
			if decodeErr != nil {
				return nil, fmt.Errorf("template XML file is invalid")
			}
			if _, forbidden := token.(xml.Directive); forbidden {
				return nil, fmt.Errorf("template XML directives are not allowed")
			}
		}
	case ".txt":
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("template TXT file must be UTF-8")
		}
	case ".png":
		if len(data) < 8 || !bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
			return nil, fmt.Errorf("template PNG file is invalid")
		}
	case ".webp":
		if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
			return nil, fmt.Errorf("template WebP file is invalid")
		}
	default:
		return nil, fmt.Errorf("template file type is not allowed")
	}
	return data, nil
}

func verifyNoCriticalOldPackageReferences(decodedDir, oldPackage string) error {
	manifestPath := filepath.Join(decodedDir, "AndroidManifest.xml")
	critical := []string{manifestPath}
	resourceFiles, _ := filepath.Glob(filepath.Join(decodedDir, "res", "*", "*.xml"))
	critical = append(critical, resourceFiles...)
	found := make([]string, 0, 5)
	for _, filename := range critical {
		content, err := os.ReadFile(filename)
		if err != nil {
			continue
		}
		contentText := string(content)
		if filename == manifestPath {
			protected, _, protectErr := protectManifestComponentNames(contentText, oldPackage)
			if protectErr != nil {
				return protectErr
			}
			contentText = protected
		}
		if strings.Contains(contentText, oldPackage) {
			relative, _ := filepath.Rel(decodedDir, filename)
			found = append(found, filepath.ToSlash(relative))
			if len(found) >= 5 {
				break
			}
		}
	}
	if len(found) > 0 {
		return fmt.Errorf("critical old package references remain: %s", strings.Join(found, ", "))
	}
	return nil
}

func verifyKeystoreCertificate(ctx context.Context, keystorePath, alias, password, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(expected) != 64 {
		return fmt.Errorf("expected signing certificate fingerprint is invalid")
	}
	command := exec.CommandContext(ctx, "keytool", "-exportcert", "-keystore", keystorePath,
		"-storepass:env", "APPFORGE_KEYSTORE_PASSWORD", "-alias", alias)
	command.Env = append(os.Environ(), "APPFORGE_KEYSTORE_PASSWORD="+password)
	certificate, err := command.Output()
	if err != nil || len(certificate) == 0 {
		return fmt.Errorf("export signing certificate failed")
	}
	digest := sha256.Sum256(certificate)
	actual := hex.EncodeToString(digest[:])
	if actual != expected {
		return fmt.Errorf("signing certificate fingerprint mismatch")
	}
	return nil
}
