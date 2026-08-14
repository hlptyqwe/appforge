package secretprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type VaultProvider struct {
	address   *url.URL
	tokenFile string
	namespace string
	client    *http.Client
}

func NewVaultProvider(address, tokenFile, namespace string, client *http.Client, allowHTTP bool) (*VaultProvider, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(address), "/"))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !(allowHTTP && parsed.Scheme == "http")) {
		return nil, errors.New("Vault address must use HTTPS")
	}
	if strings.TrimSpace(tokenFile) == "" {
		return nil, errors.New("Vault token file is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &VaultProvider{address: parsed, tokenFile: tokenFile, namespace: strings.TrimSpace(namespace), client: client}, nil
}

func (p *VaultProvider) Scheme() string { return "vault" }

func (p *VaultProvider) Resolve(ctx context.Context, reference *url.URL) ([]byte, error) {
	secretPath := strings.Trim(reference.Host+"/"+strings.TrimPrefix(reference.Path, "/"), "/")
	if secretPath == "" || strings.Contains(secretPath, "..") {
		return nil, errors.New("Vault secret path is invalid")
	}
	token, err := os.ReadFile(p.tokenFile)
	if err != nil || len(token) == 0 || len(token) > 16<<10 {
		return nil, errors.New("read Vault token file failed")
	}
	defer Zero(token)
	target := *p.address
	target.Path = strings.TrimRight(target.Path, "/") + "/v1/" + secretPath
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Vault-Token", strings.TrimSpace(string(token)))
	if p.namespace != "" {
		request.Header.Set("X-Vault-Namespace", p.namespace)
	}
	response, err := p.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, defaultMaximumBytes+1))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Vault returned HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || len(envelope.Data) == 0 {
		return nil, errors.New("Vault response has no data")
	}
	var kv2 struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(envelope.Data, &kv2) == nil && len(kv2.Data) > 0 {
		return kv2.Data, nil
	}
	return envelope.Data, nil
}
