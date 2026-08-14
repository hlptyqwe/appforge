// Package sourceoauth implements the trusted GitHub/GitLab OAuth boundary.
// Provider tokens are passed directly to Core RPC for encrypted persistence and
// are never returned to the browser or written to logs.
package sourceoauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"appforge/admin-api/internal/config"
	"appforge/admin-api/internal/svc"
	"appforge/common/utils"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const stateIssuer = "appforge-source-oauth"

type stateData struct {
	TenantID int64  `json:"tenantId"`
	Platform int32  `json:"platform"`
	Nonce    string `json:"nonce"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

type providerIdentity struct {
	ID       string
	Username string
}

// AvailableRepository is provider metadata that may be explicitly authorized by a tenant user.
type AvailableRepository struct {
	ExternalRepositoryID string
	RepositoryFullName   string
	DefaultBranch        string
}

func Begin(ctx context.Context, svcCtx *svc.ServiceContext, platform core.SourcePlatform) (string, error) {
	provider, scope, err := providerConfig(svcCtx.Config.SourceOAuth, platform)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(provider.ClientId) == "" || strings.TrimSpace(provider.ClientSecret) == "" {
		return "", status.Error(codes.FailedPrecondition, "source OAuth provider is not configured")
	}
	tenant, err := utils.GetTrustedTenantIdFromCtx(ctx)
	if err != nil || tenant <= 0 {
		return "", status.Error(codes.InvalidArgument, "tenant context is required")
	}
	userID, err := utils.GetUserIdFromCtx(ctx)
	if err != nil || userID <= 0 {
		return "", status.Error(codes.InvalidArgument, "user context is required")
	}
	nonce := make([]byte, 18)
	if _, err := rand.Read(nonce); err != nil {
		return "", status.Errorf(codes.Internal, "generate OAuth state failed: %v", err)
	}
	expand, _ := json.Marshal(stateData{TenantID: tenant, Platform: int32(platform), Nonce: base64.RawURLEncoding.EncodeToString(nonce)})
	state, err := utils.GenToken(svcCtx.Config.Jwt.AccessSecret, userID, "", string(expand), stateIssuer, 10*time.Minute)
	if err != nil {
		return "", status.Errorf(codes.Internal, "sign OAuth state failed: %v", err)
	}
	authorizeURL, err := url.Parse(provider.AuthorizeURL)
	if err != nil || authorizeURL.Scheme == "" || authorizeURL.Host == "" {
		return "", status.Error(codes.FailedPrecondition, "source OAuth authorize URL is invalid")
	}
	query := authorizeURL.Query()
	query.Set("client_id", provider.ClientId)
	query.Set("redirect_uri", provider.RedirectURL)
	query.Set("response_type", "code")
	query.Set("scope", scope)
	query.Set("state", state)
	authorizeURL.RawQuery = query.Encode()
	return authorizeURL.String(), nil
}

func Complete(ctx context.Context, svcCtx *svc.ServiceContext, code, signedState string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(signedState) == "" {
		return status.Error(codes.InvalidArgument, "OAuth code and state are required")
	}
	claims, err := utils.ParseToken(svcCtx.Config.Jwt.AccessSecret, signedState)
	if err != nil || claims.Issuer != stateIssuer || claims.UserId <= 0 {
		return status.Error(codes.PermissionDenied, "OAuth state is invalid or expired")
	}
	var state stateData
	if err := json.Unmarshal([]byte(claims.Expand), &state); err != nil || state.TenantID <= 0 || state.Nonce == "" {
		return status.Error(codes.PermissionDenied, "OAuth state is invalid")
	}
	platform := core.SourcePlatform(state.Platform)
	provider, _, err := providerConfig(svcCtx.Config.SourceOAuth, platform)
	if err != nil {
		return err
	}
	token, err := exchangeToken(ctx, provider, code)
	if err != nil {
		return err
	}
	identity, err := fetchIdentity(ctx, provider, platform, token.AccessToken)
	if err != nil {
		return err
	}
	expiresAt := int64(0)
	if token.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).UnixMilli()
	}
	rpcCtx := context.WithValue(ctx, utils.CtxKeyTenantId, state.TenantID)
	rpcCtx = context.WithValue(rpcCtx, utils.CtxKeyUid, claims.UserId)
	_, err = svcCtx.CoreCli.CompleteSourceIntegration(rpcCtx, &core.CompleteSourceIntegrationReq{
		Platform: platform, IntegrationName: providerName(platform) + " @" + identity.Username,
		InstallationRef: identity.ID, AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, TokenExpiresAt: expiresAt,
	})
	return err
}

func RedirectURL(cfg config.SourceOAuthConfig, platform core.SourcePlatform, succeeded bool) string {
	target, err := url.Parse(strings.TrimSpace(cfg.SuccessRedirect))
	if err != nil || target.Scheme == "" || target.Host == "" {
		return "/"
	}
	query := target.Query()
	query.Set("source_platform", strconv.Itoa(int(platform)))
	if succeeded {
		query.Set("source_connected", "1")
	} else {
		query.Set("source_error", "oauth_failed")
	}
	target.RawQuery = query.Encode()
	return target.String()
}

func PlatformFromState(secret, signedState string) core.SourcePlatform {
	claims, err := utils.ParseToken(secret, signedState)
	if err != nil || claims.Issuer != stateIssuer {
		return core.SourcePlatform_SOURCE_PLATFORM_UNKNOWN
	}
	var state stateData
	if json.Unmarshal([]byte(claims.Expand), &state) != nil {
		return core.SourcePlatform_SOURCE_PLATFORM_UNKNOWN
	}
	return core.SourcePlatform(state.Platform)
}

// ListRepositories returns only repositories visible to the stored provider token.
// The token itself remains inside this trusted service boundary.
func ListRepositories(ctx context.Context, svcCtx *svc.ServiceContext, integrationID int64) ([]AvailableRepository, error) {
	credential, err := svcCtx.CoreCli.GetSourceIntegrationCredential(ctx, &core.SourceIntegrationIdReq{Id: integrationID})
	if err != nil {
		return nil, err
	}
	if credential.GetData().GetIntegration() == nil {
		return nil, status.Error(codes.NotFound, "source integration credential not found")
	}
	integration := credential.Data.Integration
	provider, _, err := providerConfig(svcCtx.Config.SourceOAuth, integration.Platform)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(strings.TrimRight(provider.ApiBaseURL, "/"))
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, status.Error(codes.FailedPrecondition, "source provider API URL is invalid")
	}
	query := target.Query()
	switch integration.Platform {
	case core.SourcePlatform_SOURCE_PLATFORM_GITHUB:
		target.Path = strings.TrimRight(target.Path, "/") + "/user/repos"
		query.Set("per_page", "100")
		query.Set("sort", "full_name")
		query.Set("affiliation", "owner,collaborator,organization_member")
	case core.SourcePlatform_SOURCE_PLATFORM_GITLAB:
		target.Path = strings.TrimRight(target.Path, "/") + "/projects"
		query.Set("per_page", "100")
		query.Set("membership", "true")
		query.Set("simple", "true")
		query.Set("order_by", "path")
		query.Set("sort", "asc")
	}
	target.RawQuery = query.Encode()
	body, err := providerGET(ctx, target.String(), credential.Data.AccessToken)
	if err != nil {
		return nil, err
	}
	var values []struct {
		ID                json.RawMessage `json:"id"`
		FullName          string          `json:"full_name"`
		PathWithNamespace string          `json:"path_with_namespace"`
		DefaultBranch     string          `json:"default_branch"`
	}
	if err := json.Unmarshal(body, &values); err != nil {
		return nil, status.Error(codes.Unavailable, "provider repository response is invalid")
	}
	result := make([]AvailableRepository, 0, len(values))
	for _, value := range values {
		name := value.FullName
		if integration.Platform == core.SourcePlatform_SOURCE_PLATFORM_GITLAB {
			name = value.PathWithNamespace
		}
		id := strings.Trim(string(value.ID), "\"")
		if id == "" || strings.TrimSpace(name) == "" {
			continue
		}
		result = append(result, AvailableRepository{ExternalRepositoryID: id, RepositoryFullName: name, DefaultBranch: value.DefaultBranch})
	}
	return result, nil
}

// AuthorizeRepository verifies the repository against the provider before persisting the allowlist entry.
func AuthorizeRepository(ctx context.Context, svcCtx *svc.ServiceContext, integrationID int64, externalRepositoryID string) (*core.SourceRepositoryResp, error) {
	repositories, err := ListRepositories(ctx, svcCtx, integrationID)
	if err != nil {
		return nil, err
	}
	for _, repository := range repositories {
		if repository.ExternalRepositoryID == strings.TrimSpace(externalRepositoryID) {
			return svcCtx.CoreCli.AuthorizeSourceRepository(ctx, &core.AuthorizeSourceRepositoryReq{IntegrationId: integrationID,
				ExternalRepositoryId: repository.ExternalRepositoryID, RepositoryFullName: repository.RepositoryFullName,
				DefaultBranch: repository.DefaultBranch})
		}
	}
	return nil, status.Error(codes.NotFound, "repository is not available to this provider installation")
}

func providerGET(ctx context.Context, target, accessToken string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "create provider request failed")
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "AppForge/1.0")
	response, err := oauthHTTPClientFactory().Do(request)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "source provider request failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, status.Error(codes.Unavailable, "read source provider response failed")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, status.Error(codes.PermissionDenied, "source provider denied repository access")
	}
	return body, nil
}

func providerConfig(cfg config.SourceOAuthConfig, platform core.SourcePlatform) (config.SourceOAuthProviderConfig, string, error) {
	switch platform {
	case core.SourcePlatform_SOURCE_PLATFORM_GITHUB:
		return cfg.GitHub, "repo read:user", nil
	case core.SourcePlatform_SOURCE_PLATFORM_GITLAB:
		return cfg.GitLab, "read_api", nil
	default:
		return config.SourceOAuthProviderConfig{}, "", status.Error(codes.InvalidArgument, "unsupported source OAuth platform")
	}
}

func exchangeToken(ctx context.Context, provider config.SourceOAuthProviderConfig, code string) (*tokenResponse, error) {
	form := url.Values{"client_id": {provider.ClientId}, "client_secret": {provider.ClientSecret}, "code": {strings.TrimSpace(code)},
		"redirect_uri": {provider.RedirectURL}, "grant_type": {"authorization_code"}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, status.Error(codes.Internal, "create OAuth token request failed")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := oauthHTTPClientFactory().Do(request)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "source OAuth token exchange failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, status.Error(codes.Unavailable, "read source OAuth response failed")
	}
	var token tokenResponse
	if json.Unmarshal(body, &token) != nil || response.StatusCode < 200 || response.StatusCode >= 300 || strings.TrimSpace(token.AccessToken) == "" {
		return nil, status.Error(codes.PermissionDenied, "source OAuth provider rejected the authorization code")
	}
	return &token, nil
}

func fetchIdentity(ctx context.Context, provider config.SourceOAuthProviderConfig, platform core.SourcePlatform, accessToken string) (*providerIdentity, error) {
	target := strings.TrimRight(provider.ApiBaseURL, "/") + "/user"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "create provider identity request failed")
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "AppForge/1.0")
	response, err := oauthHTTPClientFactory().Do(request)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "load provider identity failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, status.Error(codes.PermissionDenied, "provider identity is unavailable")
	}
	var value struct {
		ID       json.RawMessage `json:"id"`
		Login    string          `json:"login"`
		Username string          `json:"username"`
	}
	if json.Unmarshal(body, &value) != nil {
		return nil, status.Error(codes.PermissionDenied, "provider identity response is invalid")
	}
	username := value.Login
	if platform == core.SourcePlatform_SOURCE_PLATFORM_GITLAB {
		username = value.Username
	}
	id := strings.Trim(string(value.ID), "\"")
	if id == "" || strings.TrimSpace(username) == "" {
		return nil, status.Error(codes.PermissionDenied, "provider identity is incomplete")
	}
	return &providerIdentity{ID: id, Username: strings.TrimSpace(username)}, nil
}

var oauthHTTPClientFactory = func() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func providerName(platform core.SourcePlatform) string {
	if platform == core.SourcePlatform_SOURCE_PLATFORM_GITHUB {
		return "GitHub"
	}
	if platform == core.SourcePlatform_SOURCE_PLATFORM_GITLAB {
		return "GitLab"
	}
	return "Source"
}
