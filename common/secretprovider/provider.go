// Package secretprovider resolves short-lived build secrets from explicitly
// configured enterprise providers. References are persisted, secret values are
// not cached and callers must erase returned values after use.
package secretprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

const defaultMaximumBytes int64 = 64 << 10

type Provider interface {
	Scheme() string
	Resolve(context.Context, *url.URL) ([]byte, error)
}

type Resolver struct {
	mu        sync.RWMutex
	providers map[string]Provider
	maxBytes  int64
}

type SigningSecret struct {
	KeystorePassword string `json:"keystorePassword"`
	KeyPassword      string `json:"keyPassword"`
}

func New(maxBytes int64, providers ...Provider) (*Resolver, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaximumBytes
	}
	resolver := &Resolver{providers: make(map[string]Provider), maxBytes: maxBytes}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		scheme := strings.ToLower(strings.TrimSpace(provider.Scheme()))
		if scheme == "" {
			return nil, errors.New("secret provider scheme is required")
		}
		if _, exists := resolver.providers[scheme]; exists {
			return nil, fmt.Errorf("duplicate secret provider scheme %q", scheme)
		}
		resolver.providers[scheme] = provider
	}
	return resolver, nil
}

func (r *Resolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(reference))
	if err != nil || parsed.Scheme == "" || parsed.User != nil {
		return nil, errors.New("secret reference is invalid")
	}
	r.mu.RLock()
	provider := r.providers[strings.ToLower(parsed.Scheme)]
	r.mu.RUnlock()
	if provider == nil {
		return nil, fmt.Errorf("secret provider %q is not configured", parsed.Scheme)
	}
	value, err := provider.Resolve(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("resolve %s secret: %w", parsed.Scheme, err)
	}
	if len(value) == 0 || int64(len(value)) > r.maxBytes {
		Zero(value)
		return nil, errors.New("resolved secret is empty or exceeds the configured limit")
	}
	return value, nil
}

func (r *Resolver) ResolveSigningSecret(ctx context.Context, reference string) (*SigningSecret, error) {
	raw, err := r.Resolve(ctx, reference)
	if err != nil {
		return nil, err
	}
	defer Zero(raw)
	var result SigningSecret
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, errors.New("signing secret payload must be strict JSON")
	}
	if result.KeystorePassword == "" || result.KeyPassword == "" || len(result.KeystorePassword) > 4096 || len(result.KeyPassword) > 4096 {
		return nil, errors.New("signing secret payload is incomplete")
	}
	return &result, nil
}

func (s *SigningSecret) Erase() {
	if s == nil {
		return
	}
	s.KeystorePassword = ""
	s.KeyPassword = ""
}

func Zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
