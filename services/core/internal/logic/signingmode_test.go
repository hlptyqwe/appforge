package logic

import (
	"database/sql"
	"testing"

	"appforge/proto/core"
	"appforge/services/core/models"
)

func TestSigningModeOfSchema112Representation(t *testing.T) {
	tests := []struct {
		name string
		item *models.TAppSigningConfig
		want core.SigningMode
	}{
		{name: "local keystore", item: &models.TAppSigningConfig{KeystoreObjectId: 7, SecretRef: sql.NullString{String: "local-file:///passwords.json", Valid: true}}, want: core.SigningMode_SIGNING_MODE_LOCAL_KEYSTORE},
		{name: "remote signer", item: &models.TAppSigningConfig{KeystoreObjectId: 0, SecretRef: sql.NullString{String: "local-file:///remote.json", Valid: true}}, want: core.SigningMode_SIGNING_MODE_REMOTE_APK_SIGNER},
		{name: "legacy empty", item: &models.TAppSigningConfig{}, want: core.SigningMode_SIGNING_MODE_LOCAL_KEYSTORE},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := signingModeOf(test.item); got != test.want {
				t.Fatalf("signingModeOf()=%v want=%v", got, test.want)
			}
		})
	}
}

func TestValidateRemoteSignerSecretReference(t *testing.T) {
	for _, valid := range []string{
		"local-file:///remote/signer.json",
		"k8s-secret:///remote/signer.json",
		"vault://secret/data/appforge/remote-signer",
		"aws-secretsmanager://prod/appforge/remote-signer?versionStage=AWSCURRENT",
	} {
		if err := validateRemoteSignerSecretReference(valid); err != nil {
			t.Fatalf("valid reference %q rejected: %v", valid, err)
		}
	}
	for _, invalid := range []string{
		"", "https://example.com/secret", "local-file:///", "local-file://user@example/secret",
		"local-file:///secret#fragment", "local-file:///secret\nother",
	} {
		if err := validateRemoteSignerSecretReference(invalid); err == nil {
			t.Fatalf("invalid reference %q accepted", invalid)
		}
	}
}
