package secretprovider

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type secretsManagerAPI interface {
	GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type AWSSecretsManagerProvider struct {
	client secretsManagerAPI
}

// NewAWSSecretsManagerProvider uses the AWS SDK v2 default credential chain,
// including workload identity/IRSA, ECS task roles and instance roles. AWS
// Secrets Manager performs KMS decryption on behalf of this authenticated call.
func NewAWSSecretsManagerProvider(ctx context.Context, region, endpoint string) (*AWSSecretsManagerProvider, error) {
	region = strings.TrimSpace(region)
	if region == "" {
		return nil, errors.New("AWS region is required")
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg, func(options *secretsmanager.Options) {
		if strings.TrimSpace(endpoint) != "" {
			options.BaseEndpoint = aws.String(strings.TrimRight(strings.TrimSpace(endpoint), "/"))
		}
	})
	return &AWSSecretsManagerProvider{client: client}, nil
}

func (p *AWSSecretsManagerProvider) Scheme() string { return "aws-secretsmanager" }

func (p *AWSSecretsManagerProvider) Resolve(ctx context.Context, reference *url.URL) ([]byte, error) {
	secretID := strings.Trim(reference.Host+"/"+strings.TrimPrefix(reference.Path, "/"), "/")
	if secretID == "" || strings.ContainsAny(secretID, "\r\n") {
		return nil, errors.New("AWS secret ID is invalid")
	}
	input := &secretsmanager.GetSecretValueInput{SecretId: aws.String(secretID)}
	if stage := strings.TrimSpace(reference.Query().Get("versionStage")); stage != "" {
		input.VersionStage = aws.String(stage)
	}
	if version := strings.TrimSpace(reference.Query().Get("versionId")); version != "" {
		input.VersionId = aws.String(version)
	}
	output, err := p.client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, err
	}
	if output.SecretString != nil {
		return []byte(*output.SecretString), nil
	}
	if len(output.SecretBinary) > 0 {
		return append([]byte(nil), output.SecretBinary...), nil
	}
	return nil, errors.New("AWS secret value is empty")
}
