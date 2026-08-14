package sourceoauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"

	"appforge/proto/core"
)

func TestVerifyGitHubSourceWebhookSignature(t *testing.T) {
	body := []byte(`{"action":"published"}`)
	mac := hmac.New(sha256.New, []byte("hook-secret"))
	_, _ = mac.Write(body)
	header := http.Header{"X-Hub-Signature-256": {"sha256=" + hex.EncodeToString(mac.Sum(nil))}}
	if err := verifySourceWebhookSignature(core.SourcePlatform_SOURCE_PLATFORM_GITHUB, "hook-secret", header, body); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(make([]byte, sha256.Size)))
	if err := verifySourceWebhookSignature(core.SourcePlatform_SOURCE_PLATFORM_GITHUB, "hook-secret", header, body); err == nil {
		t.Fatal("invalid signature was accepted")
	}
}

func TestNormalizeGitHubReleaseUsesExactArtifactSelector(t *testing.T) {
	trigger := &core.SourceBuildTrigger{EventType: core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_RELEASE_PUBLISHED,
		ArtifactSelector: "app-release.apk"}
	body := []byte(`{"action":"published","repository":{"id":42},"release":{"tag_name":"v1.2.3","target_commitish":"0123456789abcdef0123456789abcdef01234567","assets":[{"id":8,"name":"notes.txt"},{"id":9,"name":"app-release.apk"}]}}`)
	result, ignored, err := normalizeGitHubWebhook(context.Background(), nil, trigger, body, "release")
	if err != nil || ignored {
		t.Fatalf("release was not normalized: ignored=%v err=%v", ignored, err)
	}
	if result.ExternalRepository != "42" || result.ExternalArtifactID != "9" || result.SourceRef != "v1.2.3" || result.ReleaseRef != "v1.2.3" {
		t.Fatalf("unexpected normalized release: %+v", result)
	}
}

func TestNormalizeGitLabPipelineUsesSuccessfulConfiguredJob(t *testing.T) {
	trigger := &core.SourceBuildTrigger{EventType: core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_CI_SUCCEEDED,
		ArtifactSelector: "package-apk"}
	body := []byte(`{"object_kind":"pipeline","project":{"id":77},"object_attributes":{"id":901,"status":"success","ref":"main","sha":"abcdef0123456789abcdef0123456789abcdef01"},"builds":[{"id":13,"name":"tests","status":"success"},{"id":14,"name":"package-apk","status":"success"}]}`)
	result, ignored, err := normalizeGitLabWebhook(trigger, body, "Pipeline Hook")
	if err != nil || ignored {
		t.Fatalf("pipeline was not normalized: ignored=%v err=%v", ignored, err)
	}
	if result.ExternalRepository != "77" || result.ExternalArtifactID != "14" || result.PipelineRef != "901" || result.JobRef != "package-apk" {
		t.Fatalf("unexpected normalized pipeline: %+v", result)
	}
}

func TestSourceWebhookArtifactSelectorRejectsAmbiguity(t *testing.T) {
	values := []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{{ID: 1, Name: "app.apk"}, {ID: 2, Name: "app.apk"}}
	if selected, ok := selectProviderArtifact(values, "app.apk"); ok || selected != "" {
		t.Fatalf("ambiguous selector was accepted: %q", selected)
	}
}
