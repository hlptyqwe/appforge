// Package secretresolver builds the enterprise Secret resolver shared by the
// Builder RPC validation endpoint and Builder Worker execution path.
package secretresolver

import (
	"context"
	"fmt"
	"strings"

	"appforge/common/secretprovider"
	"appforge/services/builder/internal/config"
)

// New creates a resolver from the configured local, Kubernetes, Vault and AWS
// providers. Secret values are resolved only by callers and are never cached.
func New(ctx context.Context, c config.Config) (*secretprovider.Resolver, error) {
	providers := make([]secretprovider.Provider, 0, 4)
	if strings.TrimSpace(c.SecretProviders.LocalRoot) != "" {
		provider, err := secretprovider.NewLocalFileProvider(c.SecretProviders.LocalRoot)
		if err != nil {
			return nil, fmt.Errorf("initialize local Secret provider: %w", err)
		}
		providers = append(providers, provider)
	}
	if strings.TrimSpace(c.SecretProviders.KubernetesRoot) != "" {
		provider, err := secretprovider.NewKubernetesFileProvider(c.SecretProviders.KubernetesRoot)
		if err != nil {
			return nil, fmt.Errorf("initialize Kubernetes Secret provider: %w", err)
		}
		providers = append(providers, provider)
	}
	if strings.TrimSpace(c.SecretProviders.Vault.Address) != "" {
		provider, err := secretprovider.NewVaultProvider(c.SecretProviders.Vault.Address,
			c.SecretProviders.Vault.TokenFile, c.SecretProviders.Vault.Namespace, nil,
			c.SecretProviders.Vault.AllowHTTP)
		if err != nil {
			return nil, fmt.Errorf("initialize Vault Secret provider: %w", err)
		}
		providers = append(providers, provider)
	}
	if strings.TrimSpace(c.SecretProviders.AWS.Region) != "" {
		provider, err := secretprovider.NewAWSSecretsManagerProvider(ctx,
			c.SecretProviders.AWS.Region, c.SecretProviders.AWS.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("initialize AWS Secrets Manager provider: %w", err)
		}
		providers = append(providers, provider)
	}
	resolver, err := secretprovider.New(c.SecretProviders.MaxSecretBytes, providers...)
	if err != nil {
		return nil, fmt.Errorf("initialize enterprise Secret resolver: %w", err)
	}
	return resolver, nil
}
