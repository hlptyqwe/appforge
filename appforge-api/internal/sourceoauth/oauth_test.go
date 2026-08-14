package sourceoauth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"appforge/admin-api/internal/config"
	"appforge/admin-api/internal/svc"
	"appforge/common/utils"
	"appforge/proto/core"
)

func TestBeginCreatesSignedTenantBoundState(t *testing.T) {
	service := &svc.ServiceContext{Config: config.Config{}}
	service.Config.Jwt.AccessSecret = "state-secret"
	service.Config.SourceOAuth.GitHub = config.SourceOAuthProviderConfig{
		ClientId: "client", ClientSecret: "secret", AuthorizeURL: "https://github.example/oauth/authorize",
		RedirectURL: "https://appforge.example/public/v1/source-oauth/callback",
	}
	ctx := context.WithValue(context.Background(), utils.CtxKeyTenantId, int64(41))
	ctx = context.WithValue(ctx, utils.CtxKeyUid, int64(9))
	authorizationURL, err := Begin(ctx, service, core.SourcePlatform_SOURCE_PLATFORM_GITHUB)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("client_id") != "client" || parsed.Query().Get("redirect_uri") == "" {
		t.Fatalf("unexpected authorization URL: %s", authorizationURL)
	}
	state := parsed.Query().Get("state")
	if PlatformFromState("state-secret", state) != core.SourcePlatform_SOURCE_PLATFORM_GITHUB {
		t.Fatal("signed state did not retain the provider")
	}
	if PlatformFromState("wrong-secret", state) != core.SourcePlatform_SOURCE_PLATFORM_UNKNOWN {
		t.Fatal("state signed with a different key was accepted")
	}
}

func TestProviderExchangeAndIdentityDoNotFollowRedirects(t *testing.T) {
	originalFactory := oauthHTTPClientFactory
	t.Cleanup(func() { oauthHTTPClientFactory = originalFactory })
	oauthHTTPClientFactory = func() *http.Client {
		return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch request.URL.Path {
			case "/token":
				if request.Method != http.MethodPost || request.FormValue("client_secret") != "secret" {
					t.Fatal("invalid token exchange request")
				}
				return oauthResponse(http.StatusOK, `{"access_token":"provider-token","token_type":"bearer","expires_in":3600}`), nil
			case "/user":
				if request.Header.Get("Authorization") != "Bearer provider-token" {
					t.Fatal("provider token was not sent as bearer authentication")
				}
				return oauthResponse(http.StatusOK, `{"id":123,"login":"octocat"}`), nil
			default:
				return oauthResponse(http.StatusNotFound, `{}`), nil
			}
		})}
	}
	provider := config.SourceOAuthProviderConfig{ClientId: "client", ClientSecret: "secret", TokenURL: "https://provider.example/token", ApiBaseURL: "https://provider.example", RedirectURL: "https://appforge.example/callback"}
	token, err := exchangeToken(context.Background(), provider, "code")
	if err != nil || token.AccessToken != "provider-token" {
		t.Fatalf("exchange failed: token=%+v err=%v", token, err)
	}
	identity, err := fetchIdentity(context.Background(), provider, core.SourcePlatform_SOURCE_PLATFORM_GITHUB, token.AccessToken)
	if err != nil || identity.ID != "123" || identity.Username != "octocat" {
		t.Fatalf("identity failed: identity=%+v err=%v", identity, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func oauthResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestRedirectURLNeverIncludesProviderSecrets(t *testing.T) {
	cfg := config.SourceOAuthConfig{SuccessRedirect: "https://appforge.example/platform/developer?tab=source"}
	redirect := RedirectURL(cfg, core.SourcePlatform_SOURCE_PLATFORM_GITLAB, false)
	if !strings.Contains(redirect, "source_error=oauth_failed") || strings.Contains(redirect, "token") {
		t.Fatalf("unexpected redirect URL: %s", redirect)
	}
}
