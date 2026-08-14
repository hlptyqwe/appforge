package secretprovider

import (
	"context"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type fakeSecretsManager struct {
	input *secretsmanager.GetSecretValueInput
}

func (f *fakeSecretsManager) GetSecretValue(_ context.Context, input *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.input = input
	return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"keystorePassword":"aws-store","keyPassword":"aws-key"}`)}, nil
}

func TestAWSSecretsManagerProviderResolvesVersionedSecret(t *testing.T) {
	client := &fakeSecretsManager{}
	provider := &AWSSecretsManagerProvider{client: client}
	resolver, err := New(0, provider)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := resolver.ResolveSigningSecret(context.Background(), "aws-secretsmanager://prod/appforge/signing?versionStage=AWSCURRENT")
	if err != nil {
		t.Fatal(err)
	}
	if aws.ToString(client.input.SecretId) != "prod/appforge/signing" || aws.ToString(client.input.VersionStage) != "AWSCURRENT" {
		t.Fatalf("unexpected GetSecretValue input: %#v", client.input)
	}
	if secret.KeystorePassword != "aws-store" || secret.KeyPassword != "aws-key" {
		t.Fatal("AWS signing secret mismatch")
	}
}

func TestAWSSecretsManagerProviderRejectsEmptyReference(t *testing.T) {
	provider := &AWSSecretsManagerProvider{client: &fakeSecretsManager{}}
	reference, _ := url.Parse("aws-secretsmanager:///")
	if _, err := provider.Resolve(context.Background(), reference); err == nil {
		t.Fatal("expected empty AWS secret ID rejection")
	}
}
