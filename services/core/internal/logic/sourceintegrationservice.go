package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func validSourcePlatform(value core.SourcePlatform) bool {
	return value == core.SourcePlatform_SOURCE_PLATFORM_GITHUB || value == core.SourcePlatform_SOURCE_PLATFORM_GITLAB
}

func mapSourceIntegration(item *models.TSourceIntegration) *core.SourceIntegration {
	if item == nil {
		return nil
	}
	return &core.SourceIntegration{Id: item.Id, TenantId: item.TenantId, Platform: core.SourcePlatform(item.Platform),
		IntegrationName: item.IntegrationName, InstallationRef: item.InstallationRef, TokenExpiresAt: timeValue(item.TokenExpiresAt),
		Status: core.SourceIntegrationStatus(item.Status), LastSyncAt: timeValue(item.LastSyncAt), CreateBy: item.CreateBy,
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime)}
}

func mapSourceRepository(item *models.TSourceRepository) *core.SourceRepository {
	if item == nil {
		return nil
	}
	return &core.SourceRepository{Id: item.Id, TenantId: item.TenantId, IntegrationId: item.IntegrationId,
		ExternalRepositoryId: item.ExternalRepositoryId, RepositoryFullName: item.RepositoryFullName,
		DefaultBranch: stringValue(item.DefaultBranch), PermissionLevel: item.PermissionLevel,
		Status: core.SourceRepositoryStatus(item.Status), CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime)}
}

func mapSourceArtifact(item *models.TSourceArtifact) *core.SourceArtifact {
	if item == nil {
		return nil
	}
	return &core.SourceArtifact{Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, VersionId: item.VersionId,
		IntegrationId: item.IntegrationId, RepositoryId: item.RepositoryId, ArtifactSource: core.SourceArtifactType(item.ArtifactSource),
		ExternalArtifactId: item.ExternalArtifactId, CommitSha: item.CommitSha, PipelineRef: stringValue(item.PipelineRef),
		JobRef: stringValue(item.JobRef), ArtifactSha256: item.ArtifactSha256, StorageObjectId: item.StorageObjectId,
		CreateTime: millis(item.CreateTime)}
}

func completeSourceIntegration(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CompleteSourceIntegrationReq) (*core.SourceIntegrationResp, error) {
	if in == nil || !validSourcePlatform(in.Platform) {
		return nil, status.Error(codes.InvalidArgument, "valid source platform is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	for _, field := range []struct {
		value, name string
		max         int
	}{
		{in.IntegrationName, "integration_name", 128}, {in.InstallationRef, "installation_ref", 255}, {in.AccessToken, "access_token", 1600},
	} {
		if err := requireText(field.value, field.name, field.max); err != nil {
			return nil, err
		}
	}
	if err := requireOptionalText(in.RefreshToken, "refresh_token", 1600); err != nil {
		return nil, err
	}
	if in.TokenExpiresAt > 0 && in.TokenExpiresAt <= time.Now().UnixMilli() {
		return nil, status.Error(codes.InvalidArgument, "token_expires_at must be in the future")
	}
	accessCiphertext, err := svcCtx.Secrets.Seal(strings.TrimSpace(in.AccessToken))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "encrypt source access token failed: %v", err)
	}
	refreshCiphertext := ""
	if strings.TrimSpace(in.RefreshToken) != "" {
		refreshCiphertext, err = svcCtx.Secrets.Seal(strings.TrimSpace(in.RefreshToken))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "encrypt source refresh token failed: %v", err)
		}
	}
	item, findErr := svcCtx.SourceIntegrationModel.FindOneByTenantIdPlatformInstallationRef(ctx, tenant, int64(in.Platform), strings.TrimSpace(in.InstallationRef))
	if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "find source integration failed: %v", findErr)
	}
	if item == nil {
		item = &models.TSourceIntegration{TenantId: tenant, Platform: int64(in.Platform), IntegrationName: strings.TrimSpace(in.IntegrationName),
			InstallationRef: strings.TrimSpace(in.InstallationRef), AccessTokenCiphertext: accessCiphertext,
			RefreshTokenCiphertext: nullString(refreshCiphertext), TokenExpiresAt: nullTime(timeFromOptionalMillis(in.TokenExpiresAt)),
			Status: int64(core.SourceIntegrationStatus_SOURCE_INTEGRATION_STATUS_ACTIVE), LastSyncAt: nullTime(time.Now()), CreateBy: actorID(ctx)}
		result, err := svcCtx.SourceIntegrationModel.Insert(ctx, item)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "create source integration failed: %v", err)
		}
		item.Id, _ = result.LastInsertId()
		item, err = svcCtx.SourceIntegrationModel.FindOne(ctx, item.Id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "load source integration failed: %v", err)
		}
	} else {
		item.IntegrationName = strings.TrimSpace(in.IntegrationName)
		item.AccessTokenCiphertext = accessCiphertext
		item.RefreshTokenCiphertext = nullString(refreshCiphertext)
		item.TokenExpiresAt = nullTime(timeFromOptionalMillis(in.TokenExpiresAt))
		item.Status = int64(core.SourceIntegrationStatus_SOURCE_INTEGRATION_STATUS_ACTIVE)
		item.LastSyncAt = nullTime(time.Now())
		if err := svcCtx.SourceIntegrationModel.Update(ctx, item); err != nil {
			return nil, status.Errorf(codes.Internal, "update source integration failed: %v", err)
		}
	}
	return &core.SourceIntegrationResp{Base: okBase(), Data: mapSourceIntegration(item)}, nil
}

func timeFromOptionalMillis(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}

func getSourceIntegration(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceIntegrationIdReq) (*core.SourceIntegrationResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := svcCtx.SourceIntegrationModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "source integration not found")
	}
	return &core.SourceIntegrationResp{Base: okBase(), Data: mapSourceIntegration(item)}, nil
}

func getSourceIntegrationCredential(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceIntegrationIdReq) (*core.SourceIntegrationCredentialResp, error) {
	response, err := getSourceIntegration(ctx, svcCtx, in)
	if err != nil {
		return nil, err
	}
	if response.Data.Status != core.SourceIntegrationStatus_SOURCE_INTEGRATION_STATUS_ACTIVE ||
		(response.Data.TokenExpiresAt > 0 && response.Data.TokenExpiresAt <= time.Now().UnixMilli()) {
		return nil, status.Error(codes.FailedPrecondition, "source integration credential is not active")
	}
	item, err := svcCtx.SourceIntegrationModel.FindOne(ctx, response.Data.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "source integration not found")
	}
	accessToken, err := svcCtx.Secrets.Open(item.AccessTokenCiphertext)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decrypt source access token failed: %v", err)
	}
	refreshToken := ""
	if item.RefreshTokenCiphertext.Valid {
		refreshToken, err = svcCtx.Secrets.Open(item.RefreshTokenCiphertext.String)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "decrypt source refresh token failed: %v", err)
		}
	}
	return &core.SourceIntegrationCredentialResp{Base: okBase(), Data: &core.SourceIntegrationCredential{
		Integration: response.Data, AccessToken: accessToken, RefreshToken: refreshToken,
	}}, nil
}

func listSourceIntegrations(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceIntegrationListReq) (*core.SourceIntegrationListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where, args := []string{"tenant_id = ?"}, []any{tenant}
	if validSourcePlatform(in.GetPlatform()) {
		where, args = append(where, "platform = ?"), append(args, int64(in.Platform))
	}
	if in.GetStatus() != core.SourceIntegrationStatus_SOURCE_INTEGRATION_STATUS_UNKNOWN {
		where, args = append(where, "status = ?"), append(args, int64(in.Status))
	}
	if keyword := strings.TrimSpace(in.GetKeyword()); keyword != "" {
		where, args = append(where, "(integration_name LIKE ? OR installation_ref LIKE ?)"), append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_source_integration WHERE "+whereSQL, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count source integrations failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.TSourceIntegration
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, `SELECT id, tenant_id, platform, integration_name, installation_ref,
access_token_ciphertext, refresh_token_ciphertext, token_expires_at, status, last_sync_at, create_by, create_time, update_time
FROM t_source_integration WHERE `+whereSQL+` AND id > ? ORDER BY id ASC LIMIT ?`, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list source integrations failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*core.SourceIntegration, 0, len(rows))
	var next int64
	for index := range rows {
		data = append(data, mapSourceIntegration(&rows[index]))
		next = rows[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.SourceIntegrationListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func disconnectSourceIntegration(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceIntegrationIdReq) (*core.SourceIntegrationResp, error) {
	response, err := getSourceIntegration(ctx, svcCtx, in)
	if err != nil {
		return nil, err
	}
	item, err := svcCtx.SourceIntegrationModel.FindOne(ctx, response.Data.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "source integration not found")
	}
	tombstone := make([]byte, 32)
	_, _ = rand.Read(tombstone)
	item.AccessTokenCiphertext, err = svcCtx.Secrets.Seal("revoked-" + hex.EncodeToString(tombstone))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "revoke source token failed: %v", err)
	}
	item.RefreshTokenCiphertext = nullString("")
	item.TokenExpiresAt = nullTime(time.Time{})
	item.Status = int64(core.SourceIntegrationStatus_SOURCE_INTEGRATION_STATUS_DISCONNECTED)
	item.LastSyncAt = nullTime(time.Now())
	if err := svcCtx.SourceIntegrationModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "disconnect source integration failed: %v", err)
	}
	return &core.SourceIntegrationResp{Base: okBase(), Data: mapSourceIntegration(item)}, nil
}

func authorizeSourceRepository(ctx context.Context, svcCtx *svc.ServiceContext, in *core.AuthorizeSourceRepositoryReq) (*core.SourceRepositoryResp, error) {
	if in == nil || in.IntegrationId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "integration_id is required")
	}
	if err := requireText(in.ExternalRepositoryId, "external_repository_id", 255); err != nil {
		return nil, err
	}
	if err := requireText(in.RepositoryFullName, "repository_full_name", 500); err != nil {
		return nil, err
	}
	if err := requireOptionalText(in.DefaultBranch, "default_branch", 255); err != nil {
		return nil, err
	}
	if strings.Contains(in.RepositoryFullName, "://") || strings.HasPrefix(in.RepositoryFullName, "/") || strings.HasSuffix(in.RepositoryFullName, "/") || !strings.Contains(in.RepositoryFullName, "/") {
		return nil, status.Error(codes.InvalidArgument, "repository_full_name must be a provider repository path")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	integration, err := svcCtx.SourceIntegrationModel.FindOne(ctx, in.IntegrationId)
	if err != nil || integration.TenantId != tenant || integration.Status != int64(core.SourceIntegrationStatus_SOURCE_INTEGRATION_STATUS_ACTIVE) {
		return nil, status.Error(codes.NotFound, "active source integration not found")
	}
	item, findErr := svcCtx.SourceRepositoryModel.FindOneByIntegrationIdExternalRepositoryId(ctx, in.IntegrationId, strings.TrimSpace(in.ExternalRepositoryId))
	if findErr != nil && findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "find source repository failed: %v", findErr)
	}
	if item == nil {
		item = &models.TSourceRepository{TenantId: tenant, IntegrationId: in.IntegrationId, ExternalRepositoryId: strings.TrimSpace(in.ExternalRepositoryId),
			RepositoryFullName: strings.TrimSpace(in.RepositoryFullName), DefaultBranch: nullString(in.DefaultBranch), PermissionLevel: "read",
			Status: int64(core.SourceRepositoryStatus_SOURCE_REPOSITORY_STATUS_AUTHORIZED)}
		result, err := svcCtx.SourceRepositoryModel.Insert(ctx, item)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "authorize source repository failed: %v", err)
		}
		item.Id, _ = result.LastInsertId()
		item, err = svcCtx.SourceRepositoryModel.FindOne(ctx, item.Id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "load source repository failed: %v", err)
		}
	} else {
		if item.TenantId != tenant {
			return nil, status.Error(codes.NotFound, "source repository not found")
		}
		item.RepositoryFullName = strings.TrimSpace(in.RepositoryFullName)
		item.DefaultBranch = nullString(in.DefaultBranch)
		item.PermissionLevel = "read"
		item.Status = int64(core.SourceRepositoryStatus_SOURCE_REPOSITORY_STATUS_AUTHORIZED)
		if err := svcCtx.SourceRepositoryModel.Update(ctx, item); err != nil {
			return nil, status.Errorf(codes.Internal, "update source repository failed: %v", err)
		}
	}
	return &core.SourceRepositoryResp{Base: okBase(), Data: mapSourceRepository(item)}, nil
}

func getSourceRepository(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceRepositoryIdReq) (*core.SourceRepositoryResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := svcCtx.SourceRepositoryModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "source repository not found")
	}
	return &core.SourceRepositoryResp{Base: okBase(), Data: mapSourceRepository(item)}, nil
}

func listSourceRepositories(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceRepositoryListReq) (*core.SourceRepositoryListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where, args := []string{"tenant_id = ?"}, []any{tenant}
	if in.GetIntegrationId() > 0 {
		where, args = append(where, "integration_id = ?"), append(args, in.IntegrationId)
	}
	if in.GetStatus() != core.SourceRepositoryStatus_SOURCE_REPOSITORY_STATUS_UNKNOWN {
		where, args = append(where, "status = ?"), append(args, int64(in.Status))
	}
	if keyword := strings.TrimSpace(in.GetKeyword()); keyword != "" {
		where, args = append(where, "repository_full_name LIKE ?"), append(args, "%"+keyword+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_source_repository WHERE "+whereSQL, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count source repositories failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.TSourceRepository
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, `SELECT id, tenant_id, integration_id, external_repository_id, repository_full_name,
default_branch, permission_level, status, create_time, update_time FROM t_source_repository WHERE `+whereSQL+` AND id > ? ORDER BY id ASC LIMIT ?`, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list source repositories failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*core.SourceRepository, 0, len(rows))
	var next int64
	for index := range rows {
		data = append(data, mapSourceRepository(&rows[index]))
		next = rows[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.SourceRepositoryListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func revokeSourceRepository(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceRepositoryIdReq) (*core.SourceRepositoryResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := svcCtx.SourceRepositoryModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "source repository not found")
	}
	item.Status = int64(core.SourceRepositoryStatus_SOURCE_REPOSITORY_STATUS_REVOKED)
	if err := svcCtx.SourceRepositoryModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "revoke source repository failed: %v", err)
	}
	return &core.SourceRepositoryResp{Base: okBase(), Data: mapSourceRepository(item)}, nil
}

func recordSourceArtifact(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RecordSourceArtifactReq) (*core.SourceArtifactResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	for _, field := range []struct {
		value int64
		name  string
	}{{in.AppId, "app_id"}, {in.VersionId, "version_id"}, {in.IntegrationId, "integration_id"}, {in.RepositoryId, "repository_id"}, {in.StorageObjectId, "storage_object_id"}} {
		if err := requirePositive(field.value, field.name); err != nil {
			return nil, err
		}
	}
	if in.ArtifactSource != core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_RELEASE && in.ArtifactSource != core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_CI_JOB {
		return nil, status.Error(codes.InvalidArgument, "valid artifact_source is required")
	}
	if err := requireText(in.ExternalArtifactId, "external_artifact_id", 255); err != nil {
		return nil, err
	}
	commit := strings.ToLower(strings.TrimSpace(in.CommitSha))
	digest := strings.ToLower(strings.TrimSpace(in.ArtifactSha256))
	if !isHexLength(commit, 7, 64) {
		return nil, status.Error(codes.InvalidArgument, "commit_sha must be a hexadecimal commit identifier")
	}
	if !isHexLength(digest, 64, 64) {
		return nil, status.Error(codes.InvalidArgument, "artifact_sha256 must be a SHA-256 digest")
	}
	if err := requireOptionalText(in.PipelineRef, "pipeline_ref", 255); err != nil {
		return nil, err
	}
	if err := requireOptionalText(in.JobRef, "job_ref", 255); err != nil {
		return nil, err
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	integration, err := svcCtx.SourceIntegrationModel.FindOne(ctx, in.IntegrationId)
	if err != nil || integration.TenantId != tenant || integration.Status != 1 {
		return nil, status.Error(codes.NotFound, "active source integration not found")
	}
	repository, err := svcCtx.SourceRepositoryModel.FindOne(ctx, in.RepositoryId)
	if err != nil || repository.TenantId != tenant || repository.IntegrationId != in.IntegrationId || repository.Status != 1 {
		return nil, status.Error(codes.NotFound, "authorized source repository not found")
	}
	app, err := svcCtx.ApplicationModel.FindOne(ctx, in.AppId)
	if err != nil || app.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "application not found")
	}
	version, err := svcCtx.VersionModel.FindOne(ctx, in.VersionId)
	if err != nil || version.TenantId != tenant || version.AppId != in.AppId || version.SourceApkObjectId != in.StorageObjectId {
		return nil, status.Error(codes.InvalidArgument, "version is not bound to the imported APK")
	}
	object, err := svcCtx.StorageObjectModel.FindOne(ctx, in.StorageObjectId)
	if err != nil || object.TenantId != tenant || object.AppId != in.AppId || (object.Status != storageStatusReady && object.Status != storageStatusBound) || !strings.EqualFold(stringValue(object.Sha256), digest) {
		return nil, status.Error(codes.InvalidArgument, "storage object is not a validated imported APK")
	}
	existing, findErr := svcCtx.SourceArtifactModel.FindOneByIntegrationIdExternalArtifactId(ctx, in.IntegrationId, strings.TrimSpace(in.ExternalArtifactId))
	if findErr == nil {
		if existing.TenantId == tenant && existing.RepositoryId == in.RepositoryId && existing.VersionId == in.VersionId && strings.EqualFold(existing.ArtifactSha256, digest) {
			return &core.SourceArtifactResp{Base: okBase(), Data: mapSourceArtifact(existing)}, nil
		}
		return nil, status.Error(codes.AlreadyExists, "external artifact is already bound to another import")
	}
	if findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "find source artifact failed: %v", findErr)
	}
	item := &models.TSourceArtifact{TenantId: tenant, AppId: in.AppId, VersionId: in.VersionId, IntegrationId: in.IntegrationId,
		RepositoryId: in.RepositoryId, ArtifactSource: int64(in.ArtifactSource), ExternalArtifactId: strings.TrimSpace(in.ExternalArtifactId),
		CommitSha: commit, PipelineRef: nullString(in.PipelineRef), JobRef: nullString(in.JobRef), ArtifactSha256: digest, StorageObjectId: in.StorageObjectId}
	result, err := svcCtx.SourceArtifactModel.Insert(ctx, item)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "record source artifact failed: %v", err)
	}
	item.Id, _ = result.LastInsertId()
	item, err = svcCtx.SourceArtifactModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load source artifact failed: %v", err)
	}
	return &core.SourceArtifactResp{Base: okBase(), Data: mapSourceArtifact(item)}, nil
}

func importSourceArtifact(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ImportSourceArtifactReq) (*core.SourceArtifactImportResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	for _, field := range []struct {
		value int64
		name  string
	}{{in.AppId, "app_id"}, {in.VersionCode, "version_code"},
		{in.IntegrationId, "integration_id"}, {in.RepositoryId, "repository_id"}, {in.StorageObjectId, "storage_object_id"}} {
		if err := requirePositive(field.value, field.name); err != nil {
			return nil, err
		}
	}
	if err := requireText(in.VersionName, "version_name", 64); err != nil {
		return nil, err
	}
	if err := requireOptionalText(in.ReleaseNotes, "release_notes", 2000); err != nil {
		return nil, err
	}
	if in.ArtifactSource != core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_RELEASE && in.ArtifactSource != core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_CI_JOB {
		return nil, status.Error(codes.InvalidArgument, "valid artifact_source is required")
	}
	if err := requireText(in.ExternalArtifactId, "external_artifact_id", 255); err != nil {
		return nil, err
	}
	commit := strings.ToLower(strings.TrimSpace(in.CommitSha))
	digest := strings.ToLower(strings.TrimSpace(in.ArtifactSha256))
	if !isHexLength(commit, 7, 64) {
		return nil, status.Error(codes.InvalidArgument, "commit_sha must be a hexadecimal commit identifier")
	}
	if !isHexLength(digest, 64, 64) {
		return nil, status.Error(codes.InvalidArgument, "artifact_sha256 must be a SHA-256 digest")
	}
	if err := requireOptionalText(in.PipelineRef, "pipeline_ref", 255); err != nil {
		return nil, err
	}
	if err := requireOptionalText(in.JobRef, "job_ref", 255); err != nil {
		return nil, err
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	app, err := svcCtx.ApplicationModel.FindOne(ctx, in.AppId)
	if err != nil || app.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "application not found")
	}
	integration, err := svcCtx.SourceIntegrationModel.FindOne(ctx, in.IntegrationId)
	if err != nil || integration.TenantId != tenant || integration.Status != 1 {
		return nil, status.Error(codes.NotFound, "active source integration not found")
	}
	repository, err := svcCtx.SourceRepositoryModel.FindOne(ctx, in.RepositoryId)
	if err != nil || repository.TenantId != tenant || repository.IntegrationId != in.IntegrationId || repository.Status != 1 {
		return nil, status.Error(codes.NotFound, "authorized source repository not found")
	}
	object, err := svcCtx.StorageObjectModel.FindOne(ctx, in.StorageObjectId)
	if err != nil || object.TenantId != tenant || (object.AppId != 0 && object.AppId != in.AppId) || object.ObjectType != int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_SOURCE_APK) || object.Status != storageStatusReady || !strings.EqualFold(stringValue(object.Sha256), digest) {
		return nil, status.Error(codes.FailedPrecondition, "storage object is not a ready imported APK")
	}
	if existing, findErr := svcCtx.SourceArtifactModel.FindOneByIntegrationIdExternalArtifactId(ctx, in.IntegrationId, strings.TrimSpace(in.ExternalArtifactId)); findErr == nil {
		if existing.TenantId != tenant || existing.AppId != in.AppId || existing.RepositoryId != in.RepositoryId ||
			existing.ArtifactSource != int64(in.ArtifactSource) || existing.CommitSha != commit || existing.ArtifactSha256 != digest {
			return nil, status.Error(codes.AlreadyExists, "external artifact was already imported with different provenance")
		}
		version, versionErr := svcCtx.VersionModel.FindOne(ctx, existing.VersionId)
		if versionErr != nil || version.TenantId != tenant || version.AppId != in.AppId || version.VersionCode != in.VersionCode ||
			version.VersionName != strings.TrimSpace(in.VersionName) {
			return nil, status.Error(codes.AlreadyExists, "external artifact was already imported with a different version")
		}
		return &core.SourceArtifactImportResp{Base: okBase(), Data: &core.SourceArtifactImportResult{
			Version: mapVersion(version), Artifact: mapSourceArtifact(existing),
		}}, nil
	} else if findErr != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check source artifact failed: %v", findErr)
	}
	if _, err := svcCtx.VersionModel.FindOneByAppIdVersionCode(ctx, in.AppId, in.VersionCode); err == nil {
		return nil, status.Error(codes.AlreadyExists, "version_code already exists")
	} else if err != models.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "check version_code failed: %v", err)
	}
	var versionID int64
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		result, err := svcCtx.VersionModel.WithSession(session).Insert(txCtx, &models.TAppVersion{TenantId: tenant, AppId: in.AppId,
			VersionCode: in.VersionCode, VersionName: strings.TrimSpace(in.VersionName), SourceApkObjectId: in.StorageObjectId,
			SourceApkUrl: nullString(object.ObjectKey), SourceApkSha256: object.Sha256, ReleaseNotes: nullString(in.ReleaseNotes),
			Status: int64(core.VersionStatus_VERSION_STATUS_DRAFT), CreateBy: actorID(ctx)})
		if err != nil {
			return status.Errorf(codes.Internal, "create imported version failed: %v", err)
		}
		versionID, err = result.LastInsertId()
		if err != nil {
			return err
		}
		object.AppId = in.AppId
		object.Status = storageStatusBound
		if err := svcCtx.StorageObjectModel.WithSession(session).Update(txCtx, object); err != nil {
			return status.Errorf(codes.Internal, "bind imported APK failed: %v", err)
		}
		_, err = svcCtx.SourceArtifactModel.WithSession(session).Insert(txCtx, &models.TSourceArtifact{TenantId: tenant, AppId: in.AppId,
			VersionId: versionID, IntegrationId: in.IntegrationId, RepositoryId: in.RepositoryId, ArtifactSource: int64(in.ArtifactSource),
			ExternalArtifactId: strings.TrimSpace(in.ExternalArtifactId), CommitSha: commit, PipelineRef: nullString(in.PipelineRef),
			JobRef: nullString(in.JobRef), ArtifactSha256: digest, StorageObjectId: in.StorageObjectId})
		if err != nil {
			return status.Errorf(codes.Internal, "record imported source artifact failed: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	version, err := svcCtx.VersionModel.FindOne(ctx, versionID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load imported version failed: %v", err)
	}
	artifact, err := svcCtx.SourceArtifactModel.FindOneByIntegrationIdExternalArtifactId(ctx, in.IntegrationId, strings.TrimSpace(in.ExternalArtifactId))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load imported source artifact failed: %v", err)
	}
	return &core.SourceArtifactImportResp{Base: okBase(), Data: &core.SourceArtifactImportResult{Version: mapVersion(version), Artifact: mapSourceArtifact(artifact)}}, nil
}

func isHexLength(value string, minLength, maxLength int) bool {
	if len(value) < minLength || len(value) > maxLength {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}
