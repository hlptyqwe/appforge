package sourceoauth

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"appforge/admin-api/internal/config"
	"appforge/admin-api/internal/svc"
	"appforge/proto/core"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxProviderArtifactBytes = int64(2 * 1024 * 1024 * 1024)

// FetchedArtifact is a verified local download plus immutable provider provenance.
type FetchedArtifact struct {
	FilePath           string
	FileName           string
	Size               int64
	SHA256             string
	CommitSHA          string
	PipelineRef        string
	JobRef             string
	ExternalArtifactID string
	IntegrationID      int64
	RepositoryID       int64
	ArtifactSource     core.SourceArtifactType
}

// FetchArtifact resolves only provider-defined Release or CI endpoints for an
// already authorized repository. It never executes repository code or accepts a download URL.
func FetchArtifact(ctx context.Context, svcCtx *svc.ServiceContext, repositoryID int64, artifactSource core.SourceArtifactType, externalArtifactID, releaseRef string) (*FetchedArtifact, error) {
	repositoryResp, err := svcCtx.CoreCli.GetSourceRepository(ctx, &core.SourceRepositoryIdReq{Id: repositoryID})
	if err != nil || repositoryResp.GetData() == nil || repositoryResp.Data.Status != core.SourceRepositoryStatus_SOURCE_REPOSITORY_STATUS_AUTHORIZED {
		return nil, status.Error(codes.NotFound, "authorized source repository not found")
	}
	repository := repositoryResp.Data
	credential, err := svcCtx.CoreCli.GetSourceIntegrationCredential(ctx, &core.SourceIntegrationIdReq{Id: repository.IntegrationId})
	if err != nil || credential.GetData().GetIntegration() == nil {
		return nil, status.Error(codes.FailedPrecondition, "source integration credential is unavailable")
	}
	provider, _, err := providerConfig(svcCtx.Config.SourceOAuth, credential.Data.Integration.Platform)
	if err != nil {
		return nil, err
	}
	externalArtifactID = strings.TrimSpace(externalArtifactID)
	if externalArtifactID == "" || len(externalArtifactID) > 255 {
		return nil, status.Error(codes.InvalidArgument, "external_artifact_id is required")
	}
	var result *FetchedArtifact
	switch credential.Data.Integration.Platform {
	case core.SourcePlatform_SOURCE_PLATFORM_GITHUB:
		result, err = fetchGitHubArtifact(ctx, provider, credential.Data.AccessToken, repository, artifactSource, externalArtifactID, releaseRef)
	case core.SourcePlatform_SOURCE_PLATFORM_GITLAB:
		result, err = fetchGitLabArtifact(ctx, provider, credential.Data.AccessToken, repository, artifactSource, externalArtifactID, releaseRef)
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported source platform")
	}
	if err != nil {
		return nil, err
	}
	result.IntegrationID = repository.IntegrationId
	result.RepositoryID = repository.Id
	result.ArtifactSource = artifactSource
	prefix := "release:"
	if artifactSource == core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_CI_JOB {
		prefix = "ci:"
	}
	result.ExternalArtifactID = prefix + externalArtifactID
	return result, nil
}

func fetchGitHubArtifact(ctx context.Context, provider config.SourceOAuthProviderConfig, token string, repository *core.SourceRepository, source core.SourceArtifactType, artifactID, releaseRef string) (*FetchedArtifact, error) {
	base := strings.TrimRight(provider.ApiBaseURL, "/") + "/repos/" + escapeRepositoryPath(repository.RepositoryFullName)
	switch source {
	case core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_RELEASE:
		if strings.TrimSpace(releaseRef) == "" {
			return nil, status.Error(codes.InvalidArgument, "release_ref is required for a Release artifact")
		}
		body, err := providerGET(ctx, base+"/releases/tags/"+url.PathEscape(strings.TrimSpace(releaseRef)), token)
		if err != nil {
			return nil, err
		}
		var release struct {
			TargetCommitish string `json:"target_commitish"`
			Assets          []struct {
				ID   json.RawMessage `json:"id"`
				Name string          `json:"name"`
				URL  string          `json:"url"`
			} `json:"assets"`
		}
		if json.Unmarshal(body, &release) != nil {
			return nil, status.Error(codes.Unavailable, "GitHub release response is invalid")
		}
		var assetURL, assetName string
		for _, asset := range release.Assets {
			if strings.Trim(string(asset.ID), "\"") == artifactID {
				assetURL, assetName = asset.URL, asset.Name
				break
			}
		}
		if assetURL == "" {
			return nil, status.Error(codes.NotFound, "GitHub release asset not found")
		}
		commit, err := resolveGitHubCommit(ctx, base, token, release.TargetCommitish)
		if err != nil {
			return nil, err
		}
		return downloadProviderAPK(ctx, provider, token, assetURL, assetName, false, commit, strings.TrimSpace(releaseRef), artifactID)
	case core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_CI_JOB:
		body, err := providerGET(ctx, base+"/actions/artifacts/"+url.PathEscape(artifactID), token)
		if err != nil {
			return nil, err
		}
		var artifact struct {
			ID                 json.RawMessage `json:"id"`
			Name               string          `json:"name"`
			ArchiveDownloadURL string          `json:"archive_download_url"`
			WorkflowRun        struct {
				ID      json.RawMessage `json:"id"`
				HeadSHA string          `json:"head_sha"`
			} `json:"workflow_run"`
		}
		if json.Unmarshal(body, &artifact) != nil || strings.Trim(string(artifact.ID), "\"") != artifactID || artifact.ArchiveDownloadURL == "" || artifact.WorkflowRun.HeadSHA == "" {
			return nil, status.Error(codes.NotFound, "GitHub Actions artifact not found")
		}
		return downloadProviderAPK(ctx, provider, token, artifact.ArchiveDownloadURL, artifact.Name+".zip", true,
			artifact.WorkflowRun.HeadSHA, strings.Trim(string(artifact.WorkflowRun.ID), "\""), artifactID)
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported artifact source")
	}
}

func resolveGitHubCommit(ctx context.Context, base, token, ref string) (string, error) {
	body, err := providerGET(ctx, base+"/commits/"+url.PathEscape(strings.TrimSpace(ref)), token)
	if err != nil {
		return "", err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if json.Unmarshal(body, &commit) != nil || !validCommitSHA(commit.SHA) {
		return "", status.Error(codes.Unavailable, "GitHub commit response is invalid")
	}
	return strings.ToLower(commit.SHA), nil
}

func fetchGitLabArtifact(ctx context.Context, provider config.SourceOAuthProviderConfig, token string, repository *core.SourceRepository, source core.SourceArtifactType, artifactID, releaseRef string) (*FetchedArtifact, error) {
	base := strings.TrimRight(provider.ApiBaseURL, "/") + "/projects/" + url.PathEscape(repository.ExternalRepositoryId)
	switch source {
	case core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_RELEASE:
		if strings.TrimSpace(releaseRef) == "" {
			return nil, status.Error(codes.InvalidArgument, "release_ref is required for a Release artifact")
		}
		body, err := providerGET(ctx, base+"/releases/"+url.PathEscape(strings.TrimSpace(releaseRef)), token)
		if err != nil {
			return nil, err
		}
		var release struct {
			TagName string `json:"tag_name"`
			Commit  struct {
				ID string `json:"id"`
			} `json:"commit"`
			Assets struct {
				Links []struct {
					ID             json.RawMessage `json:"id"`
					Name           string          `json:"name"`
					DirectAssetURL string          `json:"direct_asset_url"`
					URL            string          `json:"url"`
				} `json:"links"`
			} `json:"assets"`
		}
		if json.Unmarshal(body, &release) != nil || !validCommitSHA(release.Commit.ID) {
			return nil, status.Error(codes.Unavailable, "GitLab release response is invalid")
		}
		var assetURL, assetName string
		for _, asset := range release.Assets.Links {
			if strings.Trim(string(asset.ID), "\"") == artifactID {
				assetURL, assetName = asset.DirectAssetURL, asset.Name
				if assetURL == "" {
					assetURL = asset.URL
				}
				break
			}
		}
		if assetURL == "" || !sameConfiguredHost(provider.ApiBaseURL, assetURL) {
			return nil, status.Error(codes.NotFound, "GitLab release asset is not a controlled same-host asset")
		}
		return downloadProviderAPK(ctx, provider, token, assetURL, assetName, strings.HasSuffix(strings.ToLower(assetName), ".zip"), release.Commit.ID, release.TagName, artifactID)
	case core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_CI_JOB:
		body, err := providerGET(ctx, base+"/jobs/"+url.PathEscape(artifactID), token)
		if err != nil {
			return nil, err
		}
		var job struct {
			ID     json.RawMessage `json:"id"`
			Name   string          `json:"name"`
			Commit struct {
				ID string `json:"id"`
			} `json:"commit"`
			Pipeline struct {
				ID json.RawMessage `json:"id"`
			} `json:"pipeline"`
		}
		if json.Unmarshal(body, &job) != nil || strings.Trim(string(job.ID), "\"") != artifactID || !validCommitSHA(job.Commit.ID) {
			return nil, status.Error(codes.NotFound, "GitLab CI job not found")
		}
		return downloadProviderAPK(ctx, provider, token, base+"/jobs/"+url.PathEscape(artifactID)+"/artifacts", job.Name+".zip", true,
			job.Commit.ID, strings.Trim(string(job.Pipeline.ID), "\""), artifactID)
	default:
		return nil, status.Error(codes.InvalidArgument, "unsupported artifact source")
	}
}

func downloadProviderAPK(ctx context.Context, provider config.SourceOAuthProviderConfig, token, target, name string, archive bool, commit, pipeline, job string) (*FetchedArtifact, error) {
	downloaded, err := downloadProviderFile(ctx, provider, token, target)
	if err != nil {
		return nil, err
	}
	if archive {
		extracted, extractErr := extractSingleAPK(downloaded)
		_ = os.Remove(downloaded)
		if extractErr != nil {
			return nil, extractErr
		}
		downloaded = extracted
		name = path.Base(strings.TrimSuffix(name, path.Ext(name))) + ".apk"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".apk") {
		_ = os.Remove(downloaded)
		return nil, status.Error(codes.InvalidArgument, "provider artifact is not an APK")
	}
	info, digest, err := hashFile(downloaded)
	if err != nil {
		_ = os.Remove(downloaded)
		return nil, err
	}
	return &FetchedArtifact{FilePath: downloaded, FileName: path.Base(name), Size: info.Size(), SHA256: digest,
		CommitSHA: strings.ToLower(commit), PipelineRef: pipeline, JobRef: job, ExternalArtifactID: job}, nil
}

func downloadProviderFile(ctx context.Context, provider config.SourceOAuthProviderConfig, token, target string) (string, error) {
	current := target
	for redirects := 0; redirects <= 3; redirects++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return "", status.Error(codes.Internal, "create provider artifact request failed")
		}
		if sameConfiguredHost(provider.ApiBaseURL, current) {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		request.Header.Set("Accept", "application/octet-stream")
		request.Header.Set("User-Agent", "AppForge/1.0")
		response, err := oauthHTTPClientFactory().Do(request)
		if err != nil {
			return "", status.Error(codes.Unavailable, "provider artifact download failed")
		}
		if response.StatusCode >= 300 && response.StatusCode < 400 {
			location := response.Header.Get("Location")
			_ = response.Body.Close()
			resolved, resolveErr := resolveProviderRedirect(current, location, provider)
			if resolveErr != nil {
				return "", resolveErr
			}
			current = resolved
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			_ = response.Body.Close()
			return "", status.Error(codes.PermissionDenied, "provider denied artifact download")
		}
		file, err := os.CreateTemp("", "appforge-provider-artifact-*")
		if err != nil {
			_ = response.Body.Close()
			return "", status.Error(codes.Internal, "create provider artifact file failed")
		}
		written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxProviderArtifactBytes+1))
		closeErr := file.Close()
		_ = response.Body.Close()
		if copyErr != nil || closeErr != nil || written <= 0 || written > maxProviderArtifactBytes {
			_ = os.Remove(file.Name())
			return "", status.Error(codes.InvalidArgument, "provider artifact is empty, too large, or incomplete")
		}
		return file.Name(), nil
	}
	return "", status.Error(codes.PermissionDenied, "provider artifact redirected too many times")
}

func resolveProviderRedirect(current, location string, provider config.SourceOAuthProviderConfig) (string, error) {
	base, err := url.Parse(current)
	if err != nil {
		return "", status.Error(codes.PermissionDenied, "provider redirect is invalid")
	}
	next, err := base.Parse(location)
	if err != nil || next.Scheme != "https" || next.User != nil || next.Hostname() == "" {
		return "", status.Error(codes.PermissionDenied, "provider redirect must be HTTPS")
	}
	host := strings.ToLower(next.Hostname())
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast()) {
		return "", status.Error(codes.PermissionDenied, "provider redirect targets a private address")
	}
	providerHost := configuredHost(provider.ApiBaseURL)
	allowed := host == providerHost || strings.HasSuffix(host, ".githubusercontent.com") || strings.HasSuffix(host, ".blob.core.windows.net") || strings.HasSuffix(host, ".amazonaws.com")
	if !allowed {
		return "", status.Error(codes.PermissionDenied, "provider redirect host is not allowed")
	}
	return next.String(), nil
}

func extractSingleAPK(archivePath string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", status.Error(codes.InvalidArgument, "CI artifact is not a valid ZIP archive")
	}
	defer reader.Close()
	var candidate *zip.File
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() && strings.HasSuffix(strings.ToLower(file.Name), ".apk") {
			if candidate != nil {
				return "", status.Error(codes.InvalidArgument, "CI artifact must contain exactly one APK")
			}
			candidate = file
		}
	}
	if candidate == nil || int64(candidate.UncompressedSize64) <= 0 || int64(candidate.UncompressedSize64) > maxProviderArtifactBytes {
		return "", status.Error(codes.InvalidArgument, "CI artifact does not contain one valid APK")
	}
	input, err := candidate.Open()
	if err != nil {
		return "", status.Error(codes.InvalidArgument, "open APK in CI artifact failed")
	}
	defer input.Close()
	output, err := os.CreateTemp("", "appforge-provider-apk-*.apk")
	if err != nil {
		return "", status.Error(codes.Internal, "create extracted APK failed")
	}
	written, copyErr := io.Copy(output, io.LimitReader(input, maxProviderArtifactBytes+1))
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil || written != int64(candidate.UncompressedSize64) {
		_ = os.Remove(output.Name())
		return "", status.Error(codes.InvalidArgument, "extract APK from CI artifact failed")
	}
	return output.Name(), nil
}

func hashFile(filePath string) (os.FileInfo, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", status.Error(codes.Internal, "open provider APK failed")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, "", status.Error(codes.Internal, "inspect provider APK failed")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, "", status.Error(codes.Internal, "hash provider APK failed")
	}
	return info, hex.EncodeToString(hasher.Sum(nil)), nil
}

func escapeRepositoryPath(fullName string) string {
	parts := strings.Split(fullName, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}
func configuredHost(raw string) string {
	parsed, _ := url.Parse(raw)
	return strings.ToLower(parsed.Hostname())
}
func sameConfiguredHost(configured, target string) bool {
	return configuredHost(configured) != "" && configuredHost(configured) == configuredHost(target)
}
func validCommitSHA(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 7 || len(value) > 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func contentTypeForAPK(name string) string {
	if value := mime.TypeByExtension(path.Ext(name)); value != "" {
		return value
	}
	return "application/vnd.android.package-archive"
}
