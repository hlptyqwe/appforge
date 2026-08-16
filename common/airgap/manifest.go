// Package airgap defines the canonical, signed AIR_GAPPED task and result
// manifests shared by the control plane and Local Agent.
package airgap

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"strings"
)

const (
	SchemaVersion      int32  = 1
	SignatureAlgorithm string = "ECDSA_P256_SHA256"
	TaskManifestName   string = "manifest.json"
	ResultManifestName string = "result.json"
	MaxManifestBytes          = 2 << 20
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,127}$`)
	sha256Pattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// Artifact describes one byte-exact file carried by an offline package.
type Artifact struct {
	Role         string `json:"role"`
	Path         string `json:"path"`
	ObjectID     int64  `json:"object_id,omitempty"`
	ObjectType   int32  `json:"object_type,omitempty"`
	OriginalName string `json:"original_name"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
}

// TaskManifest is signed by the control-plane Agent CA.
type TaskManifest struct {
	SchemaVersion          int32           `json:"schema_version"`
	PackageCode            string          `json:"package_code"`
	Nonce                  string          `json:"nonce"`
	TenantID               int64           `json:"tenant_id"`
	AgentID                int64           `json:"agent_id"`
	AgentCertificateSerial string          `json:"agent_certificate_serial"`
	TaskID                 int64           `json:"task_id"`
	BuilderAttempt         int32           `json:"builder_attempt"`
	IssuedAt               int64           `json:"issued_at"`
	ExpiresAt              int64           `json:"expires_at"`
	Bundle                 json.RawMessage `json:"bundle"`
	Inputs                 []Artifact      `json:"inputs"`
}

// Signature contains one fixed-algorithm ASN.1 DER ECDSA signature.
type Signature struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// TaskEnvelope is the only allowed manifest.json representation.
type TaskEnvelope struct {
	Manifest  TaskManifest `json:"manifest"`
	Signature Signature    `json:"signature"`
}

// ResultManifest is signed by the Agent certificate private key.
type ResultManifest struct {
	SchemaVersion          int32      `json:"schema_version"`
	PackageCode            string     `json:"package_code"`
	Nonce                  string     `json:"nonce"`
	TenantID               int64      `json:"tenant_id"`
	AgentID                int64      `json:"agent_id"`
	AgentCertificateSerial string     `json:"agent_certificate_serial"`
	TaskID                 int64      `json:"task_id"`
	BuilderAttempt         int32      `json:"builder_attempt"`
	ExportPackageSHA256    string     `json:"export_package_sha256"`
	BuiltAt                int64      `json:"built_at"`
	Status                 string     `json:"status"`
	ErrorMessage           string     `json:"error_message,omitempty"`
	Outputs                []Artifact `json:"outputs,omitempty"`
}

// ResultEnvelope is the only allowed result.json representation.
type ResultEnvelope struct {
	Manifest       ResultManifest `json:"manifest"`
	CertificatePEM string         `json:"certificate_pem"`
	Signature      Signature      `json:"signature"`
}

// CanonicalJSON encodes a manifest without whitespace using stable struct field
// order. Maps are intentionally absent from signed metadata.
func CanonicalJSON(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 || len(encoded) > MaxManifestBytes {
		return nil, errors.New("AIR_GAPPED manifest size is invalid")
	}
	return encoded, nil
}

// DecodeTaskEnvelope rejects unknown fields, trailing data and non-canonical
// JSON before validating the signed identity and Artifact list.
func DecodeTaskEnvelope(data []byte) (*TaskEnvelope, error) {
	var envelope TaskEnvelope
	if err := decodeCanonical(data, &envelope); err != nil {
		return nil, err
	}
	if err := ValidateTaskManifest(&envelope.Manifest); err != nil {
		return nil, err
	}
	if err := validateSignature(envelope.Signature); err != nil {
		return nil, err
	}
	return &envelope, nil
}

// DecodeTaskManifest validates a canonical unsigned task manifest before the
// control plane signs it.
func DecodeTaskManifest(data []byte) (*TaskManifest, error) {
	var manifest TaskManifest
	if err := decodeCanonical(data, &manifest); err != nil {
		return nil, err
	}
	if err := ValidateTaskManifest(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// DecodeResultEnvelope performs the corresponding strict result validation.
func DecodeResultEnvelope(data []byte) (*ResultEnvelope, error) {
	var envelope ResultEnvelope
	if err := decodeCanonical(data, &envelope); err != nil {
		return nil, err
	}
	if strings.TrimSpace(envelope.CertificatePEM) == "" || len(envelope.CertificatePEM) > 32<<10 {
		return nil, errors.New("AIR_GAPPED Agent certificate is invalid")
	}
	if err := ValidateResultManifest(&envelope.Manifest); err != nil {
		return nil, err
	}
	if err := validateSignature(envelope.Signature); err != nil {
		return nil, err
	}
	return &envelope, nil
}

// DecodeResultManifest validates the canonical Agent-signed result metadata.
func DecodeResultManifest(data []byte) (*ResultManifest, error) {
	var manifest ResultManifest
	if err := decodeCanonical(data, &manifest); err != nil {
		return nil, err
	}
	if err := ValidateResultManifest(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func decodeCanonical(data []byte, target any) error {
	if len(data) == 0 || len(data) > MaxManifestBytes {
		return errors.New("AIR_GAPPED manifest size is invalid")
	}
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(data), MaxManifestBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("AIR_GAPPED manifest is not strict JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("AIR_GAPPED manifest contains trailing data")
	}
	canonical, err := CanonicalJSON(target)
	if err != nil || !bytes.Equal(canonical, data) {
		return errors.New("AIR_GAPPED manifest is not canonical JSON")
	}
	return nil
}

// ValidateTaskManifest validates all fields that are safe to check without
// database state or package bytes.
func ValidateTaskManifest(manifest *TaskManifest) error {
	if manifest == nil || manifest.SchemaVersion != SchemaVersion || !identifierPattern.MatchString(manifest.PackageCode) ||
		!identifierPattern.MatchString(manifest.Nonce) || manifest.TenantID <= 0 || manifest.AgentID <= 0 ||
		manifest.TaskID <= 0 || manifest.BuilderAttempt <= 0 || manifest.IssuedAt <= 0 || manifest.ExpiresAt <= manifest.IssuedAt ||
		strings.TrimSpace(manifest.AgentCertificateSerial) == "" || len(manifest.AgentCertificateSerial) > 128 ||
		len(manifest.Bundle) == 0 || !json.Valid(manifest.Bundle) || len(manifest.Inputs) == 0 {
		return errors.New("AIR_GAPPED task manifest identity is invalid")
	}
	if err := validateArtifacts(manifest.Inputs, "inputs/", true); err != nil {
		return err
	}
	counts := map[string]int{}
	for _, artifact := range manifest.Inputs {
		switch artifact.Role {
		case "source_apk", "keystore", "brand_logo", "brand_splash", "template_file":
			counts[artifact.Role]++
		default:
			return fmt.Errorf("AIR_GAPPED unsupported input role %q", artifact.Role)
		}
	}
	if counts["source_apk"] != 1 || counts["keystore"] != 1 || counts["brand_logo"] > 1 || counts["brand_splash"] > 1 {
		return errors.New("AIR_GAPPED task input cardinality is invalid")
	}
	return nil
}

// ValidateResultManifest validates success/failure output structure.
func ValidateResultManifest(manifest *ResultManifest) error {
	if manifest == nil || manifest.SchemaVersion != SchemaVersion || !identifierPattern.MatchString(manifest.PackageCode) ||
		!identifierPattern.MatchString(manifest.Nonce) || manifest.TenantID <= 0 || manifest.AgentID <= 0 ||
		manifest.TaskID <= 0 || manifest.BuilderAttempt <= 0 || manifest.BuiltAt <= 0 ||
		strings.TrimSpace(manifest.AgentCertificateSerial) == "" || len(manifest.AgentCertificateSerial) > 128 ||
		!sha256Pattern.MatchString(manifest.ExportPackageSHA256) || len(manifest.ErrorMessage) > 2000 ||
		strings.ContainsAny(manifest.ErrorMessage, "\x00\r") {
		return errors.New("AIR_GAPPED result manifest identity is invalid")
	}
	switch manifest.Status {
	case "SUCCESS":
		if manifest.ErrorMessage != "" || len(manifest.Outputs) == 0 {
			return errors.New("AIR_GAPPED successful result is incomplete")
		}
	case "FAILED":
		if strings.TrimSpace(manifest.ErrorMessage) == "" {
			return errors.New("AIR_GAPPED failed result requires an error summary")
		}
	default:
		return errors.New("AIR_GAPPED result status is invalid")
	}
	if err := validateArtifacts(manifest.Outputs, "outputs/", false); err != nil {
		return err
	}
	counts := map[string]int{}
	for _, artifact := range manifest.Outputs {
		switch artifact.Role {
		case "built_apk":
			if artifact.Path != "outputs/built.apk" {
				return errors.New("AIR_GAPPED built APK path is invalid")
			}
		case "build_log":
			if artifact.Path != "outputs/build.log" {
				return errors.New("AIR_GAPPED build log path is invalid")
			}
		default:
			return fmt.Errorf("AIR_GAPPED unsupported output role %q", artifact.Role)
		}
		counts[artifact.Role]++
	}
	if counts["built_apk"] > 1 || counts["build_log"] > 1 ||
		(manifest.Status == "SUCCESS" && counts["built_apk"] != 1) ||
		(manifest.Status == "FAILED" && counts["built_apk"] != 0) {
		return errors.New("AIR_GAPPED result output cardinality is invalid")
	}
	return nil
}

func validateArtifacts(items []Artifact, prefix string, requireSource bool) error {
	seenRole := map[string]struct{}{}
	seenPath := map[string]struct{}{}
	hasSource := false
	for _, item := range items {
		role := strings.TrimSpace(item.Role)
		if role == "" || len(role) > 64 || strings.TrimSpace(item.Path) != item.Path ||
			!strings.HasPrefix(item.Path, prefix) || strings.Contains(item.Path, "..") || strings.ContainsAny(item.Path, "\\\r\n\x00") ||
			item.SizeBytes <= 0 || !sha256Pattern.MatchString(item.SHA256) || strings.TrimSpace(item.OriginalName) == "" ||
			len(item.OriginalName) > 255 || strings.ContainsAny(item.OriginalName, "/\\\r\n\x00") || len(item.ContentType) > 255 {
			return errors.New("AIR_GAPPED Artifact metadata is invalid")
		}
		if _, ok := seenRole[role]; ok && !(prefix == "inputs/" && role == "template_file") {
			return fmt.Errorf("AIR_GAPPED duplicate Artifact role %q", role)
		}
		if _, ok := seenPath[item.Path]; ok {
			return fmt.Errorf("AIR_GAPPED duplicate Artifact path %q", item.Path)
		}
		seenRole[role], seenPath[item.Path] = struct{}{}, struct{}{}
		if role == "source_apk" {
			hasSource = true
		}
	}
	if requireSource && !hasSource {
		return errors.New("AIR_GAPPED task package lacks source APK")
	}
	return nil
}

// Sign creates the fixed-algorithm ASN.1 DER signature for canonical bytes.
func Sign(privateKey *ecdsa.PrivateKey, canonical []byte) (Signature, error) {
	if privateKey == nil || privateKey.Curve == nil || privateKey.Curve.Params().Name != "P-256" || len(canonical) == 0 {
		return Signature{}, errors.New("AIR_GAPPED signing key or payload is invalid")
	}
	digest := sha256.Sum256(canonical)
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, digest[:])
	if err != nil {
		return Signature{}, err
	}
	halfOrder := new(big.Int).Rsh(new(big.Int).Set(privateKey.Params().N), 1)
	if s.Cmp(halfOrder) > 0 {
		s.Sub(privateKey.Params().N, s)
	}
	value, err := asn1.Marshal(struct{ R, S *big.Int }{R: r, S: s})
	if err != nil {
		return Signature{}, err
	}
	return Signature{Algorithm: SignatureAlgorithm, Value: base64.StdEncoding.EncodeToString(value)}, nil
}

// Verify checks the fixed signature and rejects malformed/high-S ECDSA values.
func Verify(publicKey *ecdsa.PublicKey, canonical []byte, signature Signature) error {
	if publicKey == nil || publicKey.Curve == nil || publicKey.Curve.Params().Name != "P-256" || len(canonical) == 0 {
		return errors.New("AIR_GAPPED verification key or payload is invalid")
	}
	if err := validateSignature(signature); err != nil {
		return err
	}
	der, _ := base64.StdEncoding.DecodeString(signature.Value)
	var parsed struct{ R, S *big.Int }
	if rest, err := asn1.Unmarshal(der, &parsed); err != nil || len(rest) != 0 || parsed.R == nil || parsed.S == nil ||
		parsed.R.Sign() <= 0 || parsed.S.Sign() <= 0 || parsed.S.Cmp(new(big.Int).Rsh(new(big.Int).Set(publicKey.Params().N), 1)) > 0 {
		return errors.New("AIR_GAPPED signature encoding is invalid")
	}
	digest := sha256.Sum256(canonical)
	if !ecdsa.VerifyASN1(publicKey, digest[:], der) {
		return errors.New("AIR_GAPPED signature verification failed")
	}
	return nil
}

func validateSignature(signature Signature) error {
	if signature.Algorithm != SignatureAlgorithm || signature.Value == "" || len(signature.Value) > 256 {
		return errors.New("AIR_GAPPED signature algorithm or value is invalid")
	}
	der, err := base64.StdEncoding.Strict().DecodeString(signature.Value)
	if err != nil || len(der) < 8 || len(der) > 80 {
		return errors.New("AIR_GAPPED signature is not strict Base64 DER")
	}
	return nil
}

// Digest returns a lowercase SHA-256 for package and Artifact binding.
func Digest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
