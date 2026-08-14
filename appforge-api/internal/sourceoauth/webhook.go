package sourceoauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"appforge/admin-api/internal/svc"
	"appforge/common/utils"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SourceWebhookReceiveResult describes a verified provider delivery without exposing signing material.
type SourceWebhookReceiveResult struct {
	EventID  int64
	Accepted bool
	Ignored  bool
}

type normalizedSourceWebhook struct {
	ProviderEventID    string
	ProviderEventType  string
	ExternalRepository string
	SourceRef          string
	CommitSHA          string
	ArtifactSource     core.SourceArtifactType
	ExternalArtifactID string
	ReleaseRef         string
	PipelineRef        string
	JobRef             string
}

// ReceiveSourceWebhook validates the provider signature before parsing or persisting a delivery.
// Only the repository, event type, ref and artifact selector stored in the predefined policy are accepted.
func ReceiveSourceWebhook(ctx context.Context, svcCtx *svc.ServiceContext, platform core.SourcePlatform, token string, header http.Header, body []byte) (*SourceWebhookReceiveResult, error) {
	if len(body) == 0 || len(body) > 2<<20 {
		return nil, status.Error(codes.InvalidArgument, "source webhook payload is empty or too large")
	}
	credential, err := svcCtx.CoreCli.ResolveSourceBuildTrigger(ctx, &core.ResolveSourceBuildTriggerReq{WebhookToken: token})
	if err != nil || credential.GetData().GetTrigger() == nil {
		return nil, status.Error(codes.NotFound, "source webhook endpoint not found")
	}
	trigger := credential.Data.Trigger
	if trigger.Platform != platform {
		return nil, status.Error(codes.NotFound, "source webhook endpoint not found")
	}
	if err := verifySourceWebhookSignature(platform, credential.Data.WebhookSecret, header, body); err != nil {
		return nil, err
	}
	rpcCtx := context.WithValue(ctx, utils.CtxKeyTenantId, trigger.TenantId)
	normalized, ignored, err := normalizeSourceWebhook(rpcCtx, svcCtx, trigger, credential.Data.ExternalRepositoryId, platform, header, body)
	if err != nil {
		return nil, err
	}
	if ignored {
		return &SourceWebhookReceiveResult{Ignored: true}, nil
	}
	digest := sha256.Sum256(body)
	response, err := svcCtx.CoreCli.EnqueueSourceWebhookEvent(rpcCtx, &core.EnqueueSourceWebhookEventReq{TriggerId: trigger.Id,
		ProviderEventId: normalized.ProviderEventID, ProviderEventType: normalized.ProviderEventType,
		ExternalRepositoryId: normalized.ExternalRepository, SourceRef: normalized.SourceRef, CommitSha: normalized.CommitSHA,
		ArtifactSource: normalized.ArtifactSource, ExternalArtifactId: normalized.ExternalArtifactID,
		ReleaseRef: normalized.ReleaseRef, PipelineRef: normalized.PipelineRef, JobRef: normalized.JobRef,
		PayloadSha256: hex.EncodeToString(digest[:])})
	if err != nil {
		return nil, err
	}
	return &SourceWebhookReceiveResult{EventID: response.Data.Event.Id, Accepted: response.Data.Accepted}, nil
}

func verifySourceWebhookSignature(platform core.SourcePlatform, secret string, header http.Header, body []byte) error {
	switch platform {
	case core.SourcePlatform_SOURCE_PLATFORM_GITHUB:
		provided := strings.TrimSpace(header.Get("X-Hub-Signature-256"))
		if !strings.HasPrefix(provided, "sha256=") {
			return status.Error(codes.Unauthenticated, "GitHub webhook signature is missing")
		}
		providedBytes, err := hex.DecodeString(strings.TrimPrefix(provided, "sha256="))
		if err != nil {
			return status.Error(codes.Unauthenticated, "GitHub webhook signature is invalid")
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		if !hmac.Equal(providedBytes, mac.Sum(nil)) {
			return status.Error(codes.Unauthenticated, "GitHub webhook signature is invalid")
		}
	case core.SourcePlatform_SOURCE_PLATFORM_GITLAB:
		if !hmac.Equal([]byte(header.Get("X-Gitlab-Token")), []byte(secret)) {
			return status.Error(codes.Unauthenticated, "GitLab webhook token is invalid")
		}
	default:
		return status.Error(codes.InvalidArgument, "unsupported source webhook platform")
	}
	return nil
}

func normalizeSourceWebhook(ctx context.Context, svcCtx *svc.ServiceContext, trigger *core.SourceBuildTrigger, externalRepository string,
	platform core.SourcePlatform, header http.Header, body []byte) (*normalizedSourceWebhook, bool, error) {
	var result *normalizedSourceWebhook
	var ignored bool
	var err error
	switch platform {
	case core.SourcePlatform_SOURCE_PLATFORM_GITHUB:
		result, ignored, err = normalizeGitHubWebhook(ctx, svcCtx, trigger, body, header.Get("X-GitHub-Event"))
		if result != nil {
			result.ProviderEventID = strings.TrimSpace(header.Get("X-GitHub-Delivery"))
		}
	case core.SourcePlatform_SOURCE_PLATFORM_GITLAB:
		result, ignored, err = normalizeGitLabWebhook(trigger, body, header.Get("X-Gitlab-Event"))
		if result != nil {
			result.ProviderEventID = strings.TrimSpace(header.Get("X-Gitlab-Event-UUID"))
			if result.ProviderEventID == "" {
				result.ProviderEventID = strings.TrimSpace(header.Get("X-Gitlab-Webhook-UUID"))
			}
		}
	}
	if err != nil || ignored {
		return result, ignored, err
	}
	if result == nil || result.ProviderEventID == "" || len(result.ProviderEventID) > 255 {
		return nil, false, status.Error(codes.InvalidArgument, "provider webhook delivery ID is required")
	}
	if result.ExternalRepository != externalRepository {
		return nil, false, status.Error(codes.PermissionDenied, "provider webhook repository is not authorized for this endpoint")
	}
	matched, matchErr := path.Match(trigger.RefPattern, result.SourceRef)
	if matchErr != nil || !matched {
		return nil, true, nil
	}
	return result, false, nil
}

func normalizeGitHubWebhook(ctx context.Context, svcCtx *svc.ServiceContext, trigger *core.SourceBuildTrigger, body []byte, eventName string) (*normalizedSourceWebhook, bool, error) {
	switch strings.ToLower(strings.TrimSpace(eventName)) {
	case "release":
		if trigger.EventType != core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_RELEASE_PUBLISHED {
			return nil, true, nil
		}
		var payload struct {
			Action     string `json:"action"`
			Repository struct {
				ID json.RawMessage `json:"id"`
			} `json:"repository"`
			Release struct {
				TagName         string `json:"tag_name"`
				TargetCommitish string `json:"target_commitish"`
				Assets          []struct {
					ID   json.RawMessage `json:"id"`
					Name string          `json:"name"`
				} `json:"assets"`
			} `json:"release"`
		}
		if json.Unmarshal(body, &payload) != nil {
			return nil, false, status.Error(codes.InvalidArgument, "GitHub release webhook payload is invalid")
		}
		if payload.Action != "published" || strings.TrimSpace(payload.Release.TagName) == "" {
			return nil, true, nil
		}
		artifactID, found := selectProviderArtifact(payload.Release.Assets, trigger.ArtifactSelector)
		if !found {
			return nil, true, nil
		}
		commit := strings.ToLower(strings.TrimSpace(payload.Release.TargetCommitish))
		if !validCommitSHA(commit) {
			commit = ""
		}
		return &normalizedSourceWebhook{ProviderEventType: "release.published", ExternalRepository: rawProviderID(payload.Repository.ID),
			SourceRef: strings.TrimSpace(payload.Release.TagName), CommitSHA: commit,
			ArtifactSource: core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_RELEASE, ExternalArtifactID: artifactID,
			ReleaseRef: strings.TrimSpace(payload.Release.TagName)}, false, nil
	case "workflow_run":
		if trigger.EventType != core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_CI_SUCCEEDED {
			return nil, true, nil
		}
		var payload struct {
			Action     string `json:"action"`
			Repository struct {
				ID json.RawMessage `json:"id"`
			} `json:"repository"`
			WorkflowRun struct {
				ID         json.RawMessage `json:"id"`
				HeadSHA    string          `json:"head_sha"`
				HeadBranch string          `json:"head_branch"`
				Conclusion string          `json:"conclusion"`
			} `json:"workflow_run"`
		}
		if json.Unmarshal(body, &payload) != nil {
			return nil, false, status.Error(codes.InvalidArgument, "GitHub workflow webhook payload is invalid")
		}
		if payload.Action != "completed" || payload.WorkflowRun.Conclusion != "success" || !validCommitSHA(payload.WorkflowRun.HeadSHA) {
			return nil, true, nil
		}
		runID := rawProviderID(payload.WorkflowRun.ID)
		artifactID, err := resolveGitHubWorkflowArtifact(ctx, svcCtx, trigger.RepositoryId, runID, trigger.ArtifactSelector)
		if err != nil {
			return nil, false, err
		}
		return &normalizedSourceWebhook{ProviderEventType: "workflow_run.completed", ExternalRepository: rawProviderID(payload.Repository.ID),
			SourceRef: strings.TrimSpace(payload.WorkflowRun.HeadBranch), CommitSHA: strings.ToLower(payload.WorkflowRun.HeadSHA),
			ArtifactSource: core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_CI_JOB, ExternalArtifactID: artifactID,
			PipelineRef: runID, JobRef: trigger.ArtifactSelector}, false, nil
	default:
		return nil, true, nil
	}
}

func normalizeGitLabWebhook(trigger *core.SourceBuildTrigger, body []byte, eventName string) (*normalizedSourceWebhook, bool, error) {
	eventName = strings.ToLower(strings.TrimSpace(eventName))
	if strings.Contains(eventName, "release") {
		if trigger.EventType != core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_RELEASE_PUBLISHED {
			return nil, true, nil
		}
		var payload struct {
			ObjectKind string `json:"object_kind"`
			Project    struct {
				ID json.RawMessage `json:"id"`
			} `json:"project"`
			Tag    string `json:"tag"`
			Commit struct {
				ID string `json:"id"`
			} `json:"commit"`
			Assets struct {
				Links []struct {
					ID   json.RawMessage `json:"id"`
					Name string          `json:"name"`
				} `json:"links"`
			} `json:"assets"`
		}
		if json.Unmarshal(body, &payload) != nil {
			return nil, false, status.Error(codes.InvalidArgument, "GitLab release webhook payload is invalid")
		}
		artifactID, found := selectProviderArtifact(payload.Assets.Links, trigger.ArtifactSelector)
		if payload.ObjectKind != "release" || strings.TrimSpace(payload.Tag) == "" || !found {
			return nil, true, nil
		}
		commit := strings.ToLower(strings.TrimSpace(payload.Commit.ID))
		if !validCommitSHA(commit) {
			commit = ""
		}
		return &normalizedSourceWebhook{ProviderEventType: "release", ExternalRepository: rawProviderID(payload.Project.ID),
			SourceRef: strings.TrimSpace(payload.Tag), CommitSHA: commit,
			ArtifactSource: core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_RELEASE, ExternalArtifactID: artifactID,
			ReleaseRef: strings.TrimSpace(payload.Tag)}, false, nil
	}
	if strings.Contains(eventName, "pipeline") {
		if trigger.EventType != core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_CI_SUCCEEDED {
			return nil, true, nil
		}
		var payload struct {
			ObjectKind string `json:"object_kind"`
			Project    struct {
				ID json.RawMessage `json:"id"`
			} `json:"project"`
			ObjectAttributes struct {
				ID     json.RawMessage `json:"id"`
				Status string          `json:"status"`
				Ref    string          `json:"ref"`
				SHA    string          `json:"sha"`
			} `json:"object_attributes"`
			Builds []struct {
				ID     json.RawMessage `json:"id"`
				Name   string          `json:"name"`
				Status string          `json:"status"`
			} `json:"builds"`
		}
		if json.Unmarshal(body, &payload) != nil {
			return nil, false, status.Error(codes.InvalidArgument, "GitLab pipeline webhook payload is invalid")
		}
		if payload.ObjectKind != "pipeline" || payload.ObjectAttributes.Status != "success" || !validCommitSHA(payload.ObjectAttributes.SHA) {
			return nil, true, nil
		}
		var artifactID string
		for _, build := range payload.Builds {
			if build.Name == trigger.ArtifactSelector && build.Status == "success" {
				if artifactID != "" {
					return nil, false, status.Error(codes.FailedPrecondition, "GitLab pipeline contains multiple matching jobs")
				}
				artifactID = rawProviderID(build.ID)
			}
		}
		if artifactID == "" {
			return nil, true, nil
		}
		return &normalizedSourceWebhook{ProviderEventType: "pipeline.success", ExternalRepository: rawProviderID(payload.Project.ID),
			SourceRef: strings.TrimSpace(payload.ObjectAttributes.Ref), CommitSHA: strings.ToLower(payload.ObjectAttributes.SHA),
			ArtifactSource: core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_CI_JOB, ExternalArtifactID: artifactID,
			PipelineRef: rawProviderID(payload.ObjectAttributes.ID), JobRef: trigger.ArtifactSelector}, false, nil
	}
	return nil, true, nil
}

func selectProviderArtifact(values any, selector string) (string, bool) {
	encoded, _ := json.Marshal(values)
	var items []struct {
		ID   json.RawMessage `json:"id"`
		Name string          `json:"name"`
	}
	if json.Unmarshal(encoded, &items) != nil {
		return "", false
	}
	var selected string
	for _, item := range items {
		if item.Name == selector {
			if selected != "" {
				return "", false
			}
			selected = rawProviderID(item.ID)
		}
	}
	return selected, selected != ""
}

func resolveGitHubWorkflowArtifact(ctx context.Context, svcCtx *svc.ServiceContext, repositoryID int64, runID, selector string) (string, error) {
	repository, err := svcCtx.CoreCli.GetSourceRepository(ctx, &core.SourceRepositoryIdReq{Id: repositoryID})
	if err != nil || repository.GetData() == nil {
		return "", status.Error(codes.NotFound, "authorized source repository not found")
	}
	credential, err := svcCtx.CoreCli.GetSourceIntegrationCredential(ctx, &core.SourceIntegrationIdReq{Id: repository.Data.IntegrationId})
	if err != nil || credential.GetData().GetIntegration() == nil || credential.Data.Integration.Platform != core.SourcePlatform_SOURCE_PLATFORM_GITHUB {
		return "", status.Error(codes.FailedPrecondition, "GitHub integration credential is unavailable")
	}
	provider, _, err := providerConfig(svcCtx.Config.SourceOAuth, core.SourcePlatform_SOURCE_PLATFORM_GITHUB)
	if err != nil {
		return "", err
	}
	target := strings.TrimRight(provider.ApiBaseURL, "/") + "/repos/" + escapeRepositoryPath(repository.Data.RepositoryFullName) +
		"/actions/runs/" + url.PathEscape(runID) + "/artifacts?per_page=100"
	body, err := providerGET(ctx, target, credential.Data.AccessToken)
	if err != nil {
		return "", err
	}
	var response struct {
		Artifacts []struct {
			ID      json.RawMessage `json:"id"`
			Name    string          `json:"name"`
			Expired bool            `json:"expired"`
		} `json:"artifacts"`
	}
	if json.Unmarshal(body, &response) != nil {
		return "", status.Error(codes.Unavailable, "GitHub workflow artifacts response is invalid")
	}
	var selected string
	for _, artifact := range response.Artifacts {
		if artifact.Name == selector && !artifact.Expired {
			if selected != "" {
				return "", status.Error(codes.FailedPrecondition, "GitHub workflow contains multiple matching artifacts")
			}
			selected = rawProviderID(artifact.ID)
		}
	}
	if selected == "" {
		return "", status.Error(codes.NotFound, "GitHub workflow artifact selector did not match")
	}
	return selected, nil
}

func rawProviderID(value json.RawMessage) string {
	trimmed := strings.TrimSpace(string(value))
	if unquoted, err := strconv.Unquote(trimmed); err == nil {
		return strings.TrimSpace(unquoted)
	}
	return trimmed
}
