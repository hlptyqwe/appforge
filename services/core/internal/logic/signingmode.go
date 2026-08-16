package logic

import (
	"net/url"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/models"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func normalizedSigningMode(mode core.SigningMode) (core.SigningMode, error) {
	if mode == core.SigningMode_SIGNING_MODE_UNSPECIFIED {
		return core.SigningMode_SIGNING_MODE_LOCAL_KEYSTORE, nil
	}
	if mode != core.SigningMode_SIGNING_MODE_LOCAL_KEYSTORE && mode != core.SigningMode_SIGNING_MODE_REMOTE_APK_SIGNER {
		return core.SigningMode_SIGNING_MODE_UNSPECIFIED, status.Error(codes.InvalidArgument, "signing_mode is invalid")
	}
	return mode, nil
}

// signingModeOf preserves schema-112 compatibility: a positive Keystore
// object is local signing, while zero Keystore plus a Secret reference is the
// normalized persistent representation of REMOTE_APK_SIGNER.
func signingModeOf(item *models.TAppSigningConfig) core.SigningMode {
	if item != nil && item.KeystoreObjectId == 0 && strings.TrimSpace(stringValue(item.SecretRef)) != "" {
		return core.SigningMode_SIGNING_MODE_REMOTE_APK_SIGNER
	}
	return core.SigningMode_SIGNING_MODE_LOCAL_KEYSTORE
}

func validateRemoteSignerSecretReference(reference string) error {
	parsed, err := url.Parse(strings.TrimSpace(reference))
	if err != nil || parsed.Scheme == "" || parsed.User != nil || parsed.Fragment != "" {
		return status.Error(codes.InvalidArgument, "remote signer secret_ref is invalid")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "local-file", "k8s-secret", "vault", "aws-secretsmanager":
	default:
		return status.Error(codes.InvalidArgument, "remote signer secret_ref provider is unsupported")
	}
	if strings.Trim(parsed.Host+parsed.Path, "/") == "" || strings.ContainsAny(reference, "\r\n") {
		return status.Error(codes.InvalidArgument, "remote signer secret_ref path is required")
	}
	return nil
}
