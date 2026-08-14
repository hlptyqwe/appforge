package secretprovider

import (
	"context"
	"os"
	"testing"
)

func TestVaultRuntimeAcceptance(t *testing.T) {
	address := os.Getenv("APPFORGE_VAULT_TEST_ADDRESS")
	tokenFile := os.Getenv("APPFORGE_VAULT_TEST_TOKEN_FILE")
	reference := os.Getenv("APPFORGE_VAULT_TEST_REFERENCE")
	if address == "" || tokenFile == "" || reference == "" {
		t.Skip("Vault runtime acceptance environment is not configured")
	}
	provider, err := NewVaultProvider(address, tokenFile, "", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := New(64<<10, provider)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := resolver.ResolveSigningSecret(context.Background(), reference)
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Erase()
	if secret.KeystorePassword == "" || secret.KeyPassword == "" {
		t.Fatal("Vault signing secret is incomplete")
	}
}

func TestAWSSecretsManagerRuntimeAcceptance(t *testing.T) {
	region := os.Getenv("APPFORGE_AWS_TEST_REGION")
	reference := os.Getenv("APPFORGE_AWS_TEST_REFERENCE")
	if region == "" || reference == "" {
		t.Skip("AWS Secrets Manager runtime acceptance environment is not configured")
	}
	provider, err := NewAWSSecretsManagerProvider(context.Background(), region, os.Getenv("APPFORGE_AWS_TEST_ENDPOINT"))
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := New(64<<10, provider)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := resolver.ResolveSigningSecret(context.Background(), reference)
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Erase()
	if secret.KeystorePassword == "" || secret.KeyPassword == "" {
		t.Fatal("AWS signing secret is incomplete")
	}
}
