package logic

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"strings"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	sourceWebhookMaxAttempts = 5
	sourceVersionCodeMax     = int64(2100000000)
)

var sourceCommitPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

func validSourceBuildTriggerEventType(value core.SourceBuildTriggerEventType) bool {
	return value == core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_RELEASE_PUBLISHED ||
		value == core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_CI_SUCCEEDED
}

func validSourceBuildTriggerStatus(value core.SourceBuildTriggerStatus) bool {
	return value == core.SourceBuildTriggerStatus_SOURCE_BUILD_TRIGGER_STATUS_ACTIVE ||
		value == core.SourceBuildTriggerStatus_SOURCE_BUILD_TRIGGER_STATUS_DISABLED
}

func randomSourceTriggerSecret(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func sourceTokenHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func decodeChannelIDs(value string) []int64 {
	var result []int64
	if json.Unmarshal([]byte(value), &result) != nil {
		return nil
	}
	return result
}

func mapSourceBuildTrigger(ctx context.Context, svcCtx *svc.ServiceContext, item *models.TSourceBuildTrigger) *core.SourceBuildTrigger {
	if item == nil {
		return nil
	}
	result := &core.SourceBuildTrigger{Id: item.Id, TenantId: item.TenantId, RepositoryId: item.RepositoryId,
		AppId: item.AppId, TriggerName: item.TriggerName, EventType: core.SourceBuildTriggerEventType(item.EventType),
		RefPattern: item.RefPattern, ArtifactSelector: item.ArtifactSelector, ChannelIds: decodeChannelIDs(item.ChannelIds),
		SigningConfigId: item.SigningConfigId, BrandingProfileId: item.BrandingProfileId,
		WhiteLabelProductId: item.WhiteLabelProductId, Priority: int32(item.Priority), PoolCode: item.PoolCode,
		VersionNamePrefix: item.VersionNamePrefix, Status: core.SourceBuildTriggerStatus(item.Status),
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime)}
	if svcCtx != nil && svcCtx.SourceRepositoryModel != nil {
		if repository, err := svcCtx.SourceRepositoryModel.FindOne(ctx, item.RepositoryId); err == nil && repository.TenantId == item.TenantId {
			result.RepositoryFullName = repository.RepositoryFullName
			if svcCtx.SourceIntegrationModel != nil {
				if integration, findErr := svcCtx.SourceIntegrationModel.FindOne(ctx, repository.IntegrationId); findErr == nil && integration.TenantId == item.TenantId {
					result.Platform = core.SourcePlatform(integration.Platform)
				}
			}
		}
	}
	return result
}

func mapSourceWebhookEvent(item *models.TSourceWebhookEvent) *core.SourceWebhookEvent {
	if item == nil {
		return nil
	}
	var buildTaskIDs []int64
	if item.BuildTaskIds.Valid {
		_ = json.Unmarshal([]byte(item.BuildTaskIds.String), &buildTaskIDs)
	}
	return &core.SourceWebhookEvent{Id: item.Id, TenantId: item.TenantId, TriggerId: item.TriggerId,
		ProviderEventId: item.ProviderEventId, ProviderEventType: item.ProviderEventType, SourceRef: item.SourceRef,
		CommitSha: item.CommitSha, ArtifactSource: core.SourceArtifactType(item.ArtifactSource),
		ExternalArtifactId: item.ExternalArtifactId, ReleaseRef: stringValue(item.ReleaseRef),
		PipelineRef: stringValue(item.PipelineRef), JobRef: stringValue(item.JobRef), PayloadSha256: item.PayloadSha256,
		VersionCode: item.VersionCode, VersionName: item.VersionName, Status: core.SourceWebhookEventStatus(item.Status),
		Attempt: int32(item.Attempt), VersionId: item.VersionId, BuildTaskIds: buildTaskIDs,
		ErrorMessage: stringValue(item.ErrorMessage), CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime)}
}

func validateSourceBuildTriggerTarget(ctx context.Context, svcCtx *svc.ServiceContext, tenant int64, repositoryID, appID int64,
	channelIDs []int64, signingConfigID, brandingProfileID, whiteLabelProductID int64) error {
	repository, err := svcCtx.SourceRepositoryModel.FindOne(ctx, repositoryID)
	if err != nil || repository.TenantId != tenant || repository.Status != int64(core.SourceRepositoryStatus_SOURCE_REPOSITORY_STATUS_AUTHORIZED) {
		return status.Error(codes.InvalidArgument, "authorized source repository is required")
	}
	integration, err := svcCtx.SourceIntegrationModel.FindOne(ctx, repository.IntegrationId)
	if err != nil || integration.TenantId != tenant || integration.Status != int64(core.SourceIntegrationStatus_SOURCE_INTEGRATION_STATUS_ACTIVE) {
		return status.Error(codes.FailedPrecondition, "source integration is not active")
	}
	app, err := svcCtx.ApplicationModel.FindOne(ctx, appID)
	if err != nil || app.TenantId != tenant {
		return status.Error(codes.InvalidArgument, "application is invalid")
	}
	if len(channelIDs) == 0 || len(channelIDs) > 100 {
		return status.Error(codes.InvalidArgument, "one to one hundred channel_ids are required")
	}
	seen := make(map[int64]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		if channelID <= 0 {
			return status.Error(codes.InvalidArgument, "channel_ids must be positive")
		}
		if _, exists := seen[channelID]; exists {
			return status.Error(codes.InvalidArgument, "channel_ids must not contain duplicates")
		}
		seen[channelID] = struct{}{}
		channel, findErr := svcCtx.ChannelModel.FindOne(ctx, channelID)
		if findErr != nil || channel.TenantId != tenant || channel.AppId != appID || channel.Status != int64(core.ChannelStatus_CHANNEL_STATUS_ENABLED) {
			return status.Error(codes.InvalidArgument, "channel is not enabled for the target application")
		}
	}
	if whiteLabelProductID > 0 {
		product, findErr := svcCtx.WhiteLabelProductModel.FindOne(ctx, whiteLabelProductID)
		if findErr != nil || product.TenantId != tenant || product.AppId != appID || product.Status != 1 {
			return status.Error(codes.InvalidArgument, "white-label product is invalid")
		}
	} else {
		signing, findErr := svcCtx.SigningConfigModel.FindOne(ctx, signingConfigID)
		if findErr != nil || signing.TenantId != tenant || signing.AppId != appID || signing.Status != 1 {
			return status.Error(codes.InvalidArgument, "signing configuration is invalid")
		}
	}
	if brandingProfileID > 0 {
		branding, findErr := svcCtx.BrandingProfileModel.FindOne(ctx, brandingProfileID)
		if findErr != nil || branding.TenantId != tenant || branding.AppId != appID || branding.Status != int64(core.BrandingProfileStatus_BRANDING_PROFILE_STATUS_ENABLED) {
			return status.Error(codes.InvalidArgument, "branding profile is invalid")
		}
	}
	return nil
}

func validateSourceBuildTriggerInput(name string, eventType core.SourceBuildTriggerEventType, refPattern, artifactSelector,
	poolCode, versionNamePrefix string, priority int32) (string, string, string, string, error) {
	if err := requireText(name, "trigger_name", 128); err != nil {
		return "", "", "", "", err
	}
	if !validSourceBuildTriggerEventType(eventType) {
		return "", "", "", "", status.Error(codes.InvalidArgument, "valid event_type is required")
	}
	pattern := strings.TrimSpace(refPattern)
	if pattern == "" {
		pattern = "*"
	}
	if len(pattern) > 255 {
		return "", "", "", "", status.Error(codes.InvalidArgument, "ref_pattern is too long")
	}
	if _, err := path.Match(pattern, "validation-ref"); err != nil {
		return "", "", "", "", status.Error(codes.InvalidArgument, "ref_pattern is not a valid glob")
	}
	if err := requireText(artifactSelector, "artifact_selector", 255); err != nil {
		return "", "", "", "", err
	}
	pool, err := normalizedBuildPool(poolCode)
	if err != nil {
		return "", "", "", "", err
	}
	prefix := strings.TrimSpace(versionNamePrefix)
	if len(prefix) > 32 {
		return "", "", "", "", status.Error(codes.InvalidArgument, "version_name_prefix is too long")
	}
	if priority < 0 || priority > 100 {
		return "", "", "", "", status.Error(codes.InvalidArgument, "priority must be between 0 and 100")
	}
	return strings.TrimSpace(name), pattern, strings.TrimSpace(artifactSelector), pool, nil
}

func createSourceBuildTrigger(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CreateSourceBuildTriggerReq) (*core.SourceBuildTriggerSecretResp, error) {
	if in == nil || in.RepositoryId <= 0 || in.AppId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "repository_id and app_id are required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	name, pattern, selector, pool, err := validateSourceBuildTriggerInput(in.TriggerName, in.EventType, in.RefPattern,
		in.ArtifactSelector, in.PoolCode, in.VersionNamePrefix, in.Priority)
	if err != nil {
		return nil, err
	}
	if err := validateSourceBuildTriggerTarget(ctx, svcCtx, tenant, in.RepositoryId, in.AppId, in.ChannelIds,
		in.SigningConfigId, in.BrandingProfileId, in.WhiteLabelProductId); err != nil {
		return nil, err
	}
	channelsJSON, _ := json.Marshal(in.ChannelIds)
	token, err := randomSourceTriggerSecret(32)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate source webhook token failed: %v", err)
	}
	secret, err := randomSourceTriggerSecret(32)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate source webhook secret failed: %v", err)
	}
	ciphertext, err := svcCtx.Secrets.Seal(secret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "encrypt source webhook secret failed: %v", err)
	}
	item := &models.TSourceBuildTrigger{TenantId: tenant, RepositoryId: in.RepositoryId, AppId: in.AppId,
		TriggerName: name, EventType: int64(in.EventType), RefPattern: pattern, ArtifactSelector: selector,
		ChannelIds: string(channelsJSON), SigningConfigId: in.SigningConfigId, BrandingProfileId: in.BrandingProfileId,
		WhiteLabelProductId: in.WhiteLabelProductId, Priority: int64(in.Priority), PoolCode: pool,
		VersionNamePrefix: strings.TrimSpace(in.VersionNamePrefix), WebhookTokenHash: sourceTokenHash(token),
		WebhookSecretCiphertext: ciphertext, Status: int64(core.SourceBuildTriggerStatus_SOURCE_BUILD_TRIGGER_STATUS_ACTIVE),
		CreateBy: actorID(ctx)}
	result, err := svcCtx.SourceBuildTriggerModel.Insert(ctx, item)
	if err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			return nil, status.Error(codes.AlreadyExists, "source build trigger name already exists")
		}
		return nil, status.Errorf(codes.Internal, "create source build trigger failed: %v", err)
	}
	item.Id, _ = result.LastInsertId()
	item, err = svcCtx.SourceBuildTriggerModel.FindOne(ctx, item.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load source build trigger failed: %v", err)
	}
	return &core.SourceBuildTriggerSecretResp{Base: okBase(), Data: &core.SourceBuildTriggerSecret{
		Trigger: mapSourceBuildTrigger(ctx, svcCtx, item), WebhookToken: token, WebhookSecret: secret,
	}}, nil
}

func getSourceBuildTrigger(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceBuildTriggerIdReq) (*core.SourceBuildTriggerResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := svcCtx.SourceBuildTriggerModel.FindOne(ctx, in.Id)
	if err != nil || item.TenantId != tenant {
		return nil, status.Error(codes.NotFound, "source build trigger not found")
	}
	return &core.SourceBuildTriggerResp{Base: okBase(), Data: mapSourceBuildTrigger(ctx, svcCtx, item)}, nil
}

func updateSourceBuildTrigger(ctx context.Context, svcCtx *svc.ServiceContext, in *core.UpdateSourceBuildTriggerReq) (*core.SourceBuildTriggerResp, error) {
	if in == nil || in.Id <= 0 || !validSourceBuildTriggerStatus(in.Status) {
		return nil, status.Error(codes.InvalidArgument, "id and valid status are required")
	}
	current, err := getSourceBuildTrigger(ctx, svcCtx, &core.SourceBuildTriggerIdReq{Id: in.Id})
	if err != nil {
		return nil, err
	}
	name, pattern, selector, pool, err := validateSourceBuildTriggerInput(in.TriggerName, in.EventType, in.RefPattern,
		in.ArtifactSelector, in.PoolCode, in.VersionNamePrefix, in.Priority)
	if err != nil {
		return nil, err
	}
	item, _ := svcCtx.SourceBuildTriggerModel.FindOne(ctx, in.Id)
	if err := validateSourceBuildTriggerTarget(ctx, svcCtx, item.TenantId, item.RepositoryId, item.AppId, in.ChannelIds,
		in.SigningConfigId, in.BrandingProfileId, in.WhiteLabelProductId); err != nil {
		return nil, err
	}
	channelsJSON, _ := json.Marshal(in.ChannelIds)
	item.TriggerName, item.EventType, item.RefPattern, item.ArtifactSelector = name, int64(in.EventType), pattern, selector
	item.ChannelIds, item.SigningConfigId = string(channelsJSON), in.SigningConfigId
	item.BrandingProfileId, item.WhiteLabelProductId = in.BrandingProfileId, in.WhiteLabelProductId
	item.Priority, item.PoolCode, item.VersionNamePrefix, item.Status = int64(in.Priority), pool, strings.TrimSpace(in.VersionNamePrefix), int64(in.Status)
	if err := svcCtx.SourceBuildTriggerModel.Update(ctx, item); err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			return nil, status.Error(codes.AlreadyExists, "source build trigger name already exists")
		}
		return nil, status.Errorf(codes.Internal, "update source build trigger failed: %v", err)
	}
	_ = current
	item, _ = svcCtx.SourceBuildTriggerModel.FindOne(ctx, item.Id)
	return &core.SourceBuildTriggerResp{Base: okBase(), Data: mapSourceBuildTrigger(ctx, svcCtx, item)}, nil
}

func listSourceBuildTriggers(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceBuildTriggerListReq) (*core.SourceBuildTriggerListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where, args := []string{"tenant_id = ?"}, []any{tenant}
	if in.GetRepositoryId() > 0 {
		where, args = append(where, "repository_id = ?"), append(args, in.RepositoryId)
	}
	if in.GetAppId() > 0 {
		where, args = append(where, "app_id = ?"), append(args, in.AppId)
	}
	if validSourceBuildTriggerStatus(in.GetStatus()) {
		where, args = append(where, "status = ?"), append(args, int64(in.Status))
	}
	if keyword := strings.TrimSpace(in.GetKeyword()); keyword != "" {
		where, args = append(where, "trigger_name LIKE ?"), append(args, "%"+keyword+"%")
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_source_build_trigger WHERE "+whereSQL, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count source build triggers failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.TSourceBuildTrigger
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, `SELECT * FROM t_source_build_trigger WHERE `+whereSQL+` AND id > ? ORDER BY id ASC LIMIT ?`, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list source build triggers failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*core.SourceBuildTrigger, 0, len(rows))
	var next int64
	for index := range rows {
		data = append(data, mapSourceBuildTrigger(ctx, svcCtx, &rows[index]))
		next = rows[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.SourceBuildTriggerListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func rotateSourceBuildTriggerSecret(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceBuildTriggerIdReq) (*core.SourceBuildTriggerSecretResp, error) {
	response, err := getSourceBuildTrigger(ctx, svcCtx, in)
	if err != nil {
		return nil, err
	}
	item, _ := svcCtx.SourceBuildTriggerModel.FindOne(ctx, response.Data.Id)
	token, err := randomSourceTriggerSecret(32)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate source webhook token failed: %v", err)
	}
	secret, err := randomSourceTriggerSecret(32)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate source webhook secret failed: %v", err)
	}
	item.WebhookTokenHash = sourceTokenHash(token)
	item.WebhookSecretCiphertext, err = svcCtx.Secrets.Seal(secret)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "encrypt source webhook secret failed: %v", err)
	}
	if err := svcCtx.SourceBuildTriggerModel.Update(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "rotate source webhook secret failed: %v", err)
	}
	item, _ = svcCtx.SourceBuildTriggerModel.FindOne(ctx, item.Id)
	return &core.SourceBuildTriggerSecretResp{Base: okBase(), Data: &core.SourceBuildTriggerSecret{
		Trigger: mapSourceBuildTrigger(ctx, svcCtx, item), WebhookToken: token, WebhookSecret: secret,
	}}, nil
}

func resolveSourceBuildTrigger(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ResolveSourceBuildTriggerReq) (*core.SourceBuildTriggerCredentialResp, error) {
	if in == nil || len(strings.TrimSpace(in.WebhookToken)) < 32 || len(in.WebhookToken) > 128 {
		return nil, status.Error(codes.NotFound, "source webhook endpoint not found")
	}
	item, err := svcCtx.SourceBuildTriggerModel.FindOneByWebhookTokenHash(ctx, sourceTokenHash(strings.TrimSpace(in.WebhookToken)))
	if err != nil || item.Status != int64(core.SourceBuildTriggerStatus_SOURCE_BUILD_TRIGGER_STATUS_ACTIVE) {
		return nil, status.Error(codes.NotFound, "source webhook endpoint not found")
	}
	repository, err := svcCtx.SourceRepositoryModel.FindOne(ctx, item.RepositoryId)
	if err != nil || repository.TenantId != item.TenantId || repository.Status != int64(core.SourceRepositoryStatus_SOURCE_REPOSITORY_STATUS_AUTHORIZED) {
		return nil, status.Error(codes.FailedPrecondition, "source repository authorization is inactive")
	}
	integration, err := svcCtx.SourceIntegrationModel.FindOne(ctx, repository.IntegrationId)
	if err != nil || integration.TenantId != item.TenantId || integration.Status != int64(core.SourceIntegrationStatus_SOURCE_INTEGRATION_STATUS_ACTIVE) {
		return nil, status.Error(codes.FailedPrecondition, "source integration is inactive")
	}
	secret, err := svcCtx.Secrets.Open(item.WebhookSecretCiphertext)
	if err != nil {
		return nil, status.Error(codes.Internal, "source webhook secret cannot be decrypted")
	}
	return &core.SourceBuildTriggerCredentialResp{Base: okBase(), Data: &core.SourceBuildTriggerCredential{
		Trigger: mapSourceBuildTrigger(ctx, svcCtx, item), WebhookSecret: secret,
		ExternalRepositoryId: repository.ExternalRepositoryId,
	}}, nil
}

func normalizedSourceVersionName(prefix, sourceRef string) string {
	value := strings.TrimSpace(prefix) + strings.TrimSpace(sourceRef)
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("._-", character) {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('-')
		}
	}
	result := strings.Trim(builder.String(), "-._")
	if result == "" {
		result = "source"
	}
	if len(result) > 64 {
		result = result[:64]
	}
	return result
}

func enqueueSourceWebhookEvent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.EnqueueSourceWebhookEventReq) (*core.EnqueueSourceWebhookEventResp, error) {
	if in == nil || in.TriggerId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "trigger_id is required")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	for _, field := range []struct {
		value, name string
		max         int
	}{
		{in.ProviderEventId, "provider_event_id", 255}, {in.ProviderEventType, "provider_event_type", 64},
		{in.ExternalRepositoryId, "external_repository_id", 255}, {in.SourceRef, "source_ref", 255},
		{in.ExternalArtifactId, "external_artifact_id", 255}, {in.PayloadSha256, "payload_sha256", 64},
	} {
		if err := requireText(field.value, field.name, field.max); err != nil {
			return nil, err
		}
	}
	if (strings.TrimSpace(in.CommitSha) != "" && !sourceCommitPattern.MatchString(in.CommitSha)) || !validSHA256(in.PayloadSha256) {
		return nil, status.Error(codes.InvalidArgument, "commit_sha or payload_sha256 is invalid")
	}
	trigger, err := svcCtx.SourceBuildTriggerModel.FindOne(ctx, in.TriggerId)
	if err != nil || trigger.TenantId != tenant || trigger.Status != int64(core.SourceBuildTriggerStatus_SOURCE_BUILD_TRIGGER_STATUS_ACTIVE) {
		return nil, status.Error(codes.NotFound, "source build trigger not found")
	}
	repository, err := svcCtx.SourceRepositoryModel.FindOne(ctx, trigger.RepositoryId)
	if err != nil || repository.TenantId != tenant || repository.Status != int64(core.SourceRepositoryStatus_SOURCE_REPOSITORY_STATUS_AUTHORIZED) ||
		repository.ExternalRepositoryId != strings.TrimSpace(in.ExternalRepositoryId) {
		return nil, status.Error(codes.PermissionDenied, "source webhook repository does not match the authorized repository")
	}
	matched, matchErr := path.Match(trigger.RefPattern, strings.TrimSpace(in.SourceRef))
	if matchErr != nil || !matched {
		return nil, status.Error(codes.FailedPrecondition, "source webhook ref does not match the predefined policy")
	}
	expectedArtifactSource := core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_RELEASE
	if core.SourceBuildTriggerEventType(trigger.EventType) == core.SourceBuildTriggerEventType_SOURCE_BUILD_TRIGGER_EVENT_TYPE_CI_SUCCEEDED {
		expectedArtifactSource = core.SourceArtifactType_SOURCE_ARTIFACT_TYPE_CI_JOB
	}
	if in.ArtifactSource != expectedArtifactSource {
		return nil, status.Error(codes.FailedPrecondition, "source webhook event type does not match the predefined policy")
	}
	providerEventID := strings.TrimSpace(in.ProviderEventId)
	if existing, findErr := findSourceWebhookEventByDelivery(ctx, svcCtx, trigger.Id, providerEventID); findErr == nil {
		return &core.EnqueueSourceWebhookEventResp{Base: okBase(), Data: &core.EnqueueSourceWebhookEventResult{
			Event: mapSourceWebhookEvent(existing), Accepted: false,
		}}, nil
	}
	var created models.TSourceWebhookEvent
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var appID int64
		if err := session.QueryRowCtx(txCtx, &appID, `SELECT id FROM t_app_application WHERE id = ? AND tenant_id = ? FOR UPDATE`, trigger.AppId, tenant); err != nil {
			return status.Error(codes.FailedPrecondition, "target application is unavailable")
		}
		var nextVersionCode int64
		if err := session.QueryRowCtx(txCtx, &nextVersionCode, `SELECT COALESCE(MAX(version_code),0)+1 FROM t_app_version WHERE tenant_id = ? AND app_id = ?`, tenant, trigger.AppId); err != nil {
			return err
		}
		if nextVersionCode <= 0 || nextVersionCode > sourceVersionCodeMax {
			return status.Error(codes.ResourceExhausted, "application versionCode range is exhausted")
		}
		result, err := session.ExecCtx(txCtx, `INSERT INTO t_source_webhook_event
(tenant_id,trigger_id,provider_event_id,provider_event_type,source_ref,commit_sha,artifact_source,external_artifact_id,
release_ref,pipeline_ref,job_ref,payload_sha256,version_code,version_name,status,attempt,next_retry_at,version_id)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,0,CURRENT_TIMESTAMP(3),0)`, tenant, trigger.Id, providerEventID,
			strings.TrimSpace(in.ProviderEventType), strings.TrimSpace(in.SourceRef), strings.ToLower(strings.TrimSpace(in.CommitSha)),
			int64(in.ArtifactSource), strings.TrimSpace(in.ExternalArtifactId), nullString(strings.TrimSpace(in.ReleaseRef)),
			nullString(strings.TrimSpace(in.PipelineRef)), nullString(strings.TrimSpace(in.JobRef)), strings.ToLower(strings.TrimSpace(in.PayloadSha256)),
			nextVersionCode, normalizedSourceVersionName(trigger.VersionNamePrefix, in.SourceRef))
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		return session.QueryRowCtx(txCtx, &created, `SELECT * FROM t_source_webhook_event WHERE id = ?`, id)
	})
	if err != nil {
		var duplicate *mysql.MySQLError
		if errors.As(err, &duplicate) && duplicate.Number == 1062 {
			existing, findErr := findSourceWebhookEventByDelivery(ctx, svcCtx, trigger.Id, providerEventID)
			if findErr == nil {
				return &core.EnqueueSourceWebhookEventResp{Base: okBase(), Data: &core.EnqueueSourceWebhookEventResult{Event: mapSourceWebhookEvent(existing), Accepted: false}}, nil
			}
		}
		return nil, status.Errorf(codes.Internal, "enqueue source webhook event failed: %v", err)
	}
	return &core.EnqueueSourceWebhookEventResp{Base: okBase(), Data: &core.EnqueueSourceWebhookEventResult{
		Event: mapSourceWebhookEvent(&created), Accepted: true,
	}}, nil
}

func findSourceWebhookEventByDelivery(ctx context.Context, svcCtx *svc.ServiceContext, triggerID int64, providerEventID string) (*models.TSourceWebhookEvent, error) {
	var item models.TSourceWebhookEvent
	err := svcCtx.DB.QueryRowCtx(ctx, &item, `SELECT * FROM t_source_webhook_event
WHERE trigger_id = ? AND provider_event_id = ? LIMIT 1`, triggerID, providerEventID)
	return &item, err
}

func claimSourceWebhookEvent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ClaimSourceWebhookEventReq) (*core.ClaimSourceWebhookEventResp, error) {
	if in == nil || strings.TrimSpace(in.WorkerId) == "" || len(in.WorkerId) > 128 {
		return nil, status.Error(codes.InvalidArgument, "valid worker_id is required")
	}
	leaseSeconds := int64(in.LeaseSeconds)
	if leaseSeconds < 30 {
		leaseSeconds = 300
	}
	if leaseSeconds > 1800 {
		leaseSeconds = 1800
	}
	var claimed *models.TSourceWebhookEvent
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var row models.TSourceWebhookEvent
		err := session.QueryRowCtx(txCtx, &row, `SELECT * FROM t_source_webhook_event
WHERE ((status = 1 AND next_retry_at <= CURRENT_TIMESTAMP(3)) OR (status = 2 AND lease_until <= CURRENT_TIMESTAMP(3)))
ORDER BY next_retry_at ASC,id ASC LIMIT 1 FOR UPDATE SKIP LOCKED`)
		if err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				return nil
			}
			return err
		}
		result, err := session.ExecCtx(txCtx, `UPDATE t_source_webhook_event SET status = 2,attempt = attempt+1,
claimed_by = ?,lease_until = DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL ? SECOND),error_message = NULL
WHERE id = ? AND attempt = ?`, strings.TrimSpace(in.WorkerId), leaseSeconds, row.Id, row.Attempt)
		if err != nil {
			return err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return nil
		}
		row.Status, row.Attempt, row.ClaimedBy = 2, row.Attempt+1, sql.NullString{String: strings.TrimSpace(in.WorkerId), Valid: true}
		claimed = &row
		return nil
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "claim source webhook event failed: %v", err)
	}
	response := &core.ClaimSourceWebhookEventResp{Base: okBase()}
	if claimed == nil {
		return response, nil
	}
	trigger, err := svcCtx.SourceBuildTriggerModel.FindOne(ctx, claimed.TriggerId)
	if err != nil || trigger.TenantId != claimed.TenantId {
		_, _ = failSourceWebhookEvent(ctx, svcCtx, &core.FailSourceWebhookEventReq{Id: claimed.Id, ErrorMessage: "source build trigger is unavailable", Retryable: false})
		return response, nil
	}
	response.Data = &core.ClaimedSourceWebhookEvent{Event: mapSourceWebhookEvent(claimed), Trigger: mapSourceBuildTrigger(ctx, svcCtx, trigger)}
	return response, nil
}

func completeSourceWebhookEvent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CompleteSourceWebhookEventReq) (*core.RespBase, error) {
	if in == nil || in.Id <= 0 || in.VersionId <= 0 || len(in.BuildTaskIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "id, version_id and build_task_ids are required")
	}
	if !sourceCommitPattern.MatchString(in.CommitSha) {
		return nil, status.Error(codes.InvalidArgument, "valid commit_sha is required")
	}
	event, err := svcCtx.SourceWebhookEventModel.FindOne(ctx, in.Id)
	if err != nil || event.Status != int64(core.SourceWebhookEventStatus_SOURCE_WEBHOOK_EVENT_STATUS_PROCESSING) {
		return nil, status.Error(codes.FailedPrecondition, "source webhook event is not processing")
	}
	if tenant, tenantErr := tenantID(ctx); tenantErr == nil && tenant != event.TenantId {
		return nil, status.Error(codes.PermissionDenied, "source webhook event belongs to another tenant")
	}
	trigger, err := svcCtx.SourceBuildTriggerModel.FindOne(ctx, event.TriggerId)
	if err != nil || trigger.TenantId != event.TenantId {
		return nil, status.Error(codes.FailedPrecondition, "source build trigger is unavailable")
	}
	version, err := svcCtx.VersionModel.FindOne(ctx, in.VersionId)
	if err != nil || version.TenantId != event.TenantId || version.AppId != trigger.AppId || version.VersionCode != event.VersionCode {
		return nil, status.Error(codes.FailedPrecondition, "imported version does not match the source webhook event")
	}
	expectedChannels := decodeChannelIDs(trigger.ChannelIds)
	if len(expectedChannels) != len(in.BuildTaskIds) {
		return nil, status.Error(codes.FailedPrecondition, "build task count does not match the predefined channels")
	}
	expected := make(map[int64]struct{}, len(expectedChannels))
	for _, channelID := range expectedChannels {
		expected[channelID] = struct{}{}
	}
	seenTasks := make(map[int64]struct{}, len(in.BuildTaskIds))
	for _, taskID := range in.BuildTaskIds {
		if _, exists := seenTasks[taskID]; exists {
			return nil, status.Error(codes.InvalidArgument, "build_task_ids contain duplicates")
		}
		seenTasks[taskID] = struct{}{}
		task, findErr := svcCtx.BuildTaskModel.FindOne(ctx, taskID)
		if findErr != nil || task.TenantId != event.TenantId || task.AppId != trigger.AppId || task.VersionId != in.VersionId ||
			!task.SourceWebhookEventId.Valid || task.SourceWebhookEventId.Int64 != event.Id {
			return nil, status.Error(codes.FailedPrecondition, "build task does not match the source webhook event")
		}
		if _, exists := expected[task.ChannelId]; !exists {
			return nil, status.Error(codes.FailedPrecondition, "build task channel is not predefined by the trigger")
		}
	}
	buildIDsJSON, _ := json.Marshal(in.BuildTaskIds)
	result, err := svcCtx.DB.ExecCtx(ctx, `UPDATE t_source_webhook_event SET status = 3,version_id = ?,build_task_ids = ?,commit_sha = ?,
claimed_by = NULL,lease_until = NULL,error_message = NULL,completed_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND status = 2`, in.VersionId, string(buildIDsJSON), strings.ToLower(in.CommitSha), in.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "complete source webhook event failed: %v", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, status.Error(codes.FailedPrecondition, "source webhook event is not processing")
	}
	return &core.RespBase{Base: okBase()}, nil
}

func failSourceWebhookEvent(ctx context.Context, svcCtx *svc.ServiceContext, in *core.FailSourceWebhookEventReq) (*core.RespBase, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	message := strings.TrimSpace(in.ErrorMessage)
	if message == "" {
		message = "source webhook processing failed"
	}
	if len(message) > 1000 {
		message = message[:1000]
	}
	var attempt int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &attempt, `SELECT attempt FROM t_source_webhook_event WHERE id = ? AND status = 2`, in.Id); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "source webhook event is not processing")
	}
	if in.Retryable && attempt < sourceWebhookMaxAttempts {
		delay := int64(1 << minInt(int(attempt), 8))
		_, err := svcCtx.DB.ExecCtx(ctx, `UPDATE t_source_webhook_event SET status = 1,claimed_by = NULL,lease_until = NULL,
error_message = ?,next_retry_at = DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL ? SECOND) WHERE id = ? AND status = 2`, message, delay, in.Id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "retry source webhook event failed: %v", err)
		}
		return &core.RespBase{Base: okBase()}, nil
	}
	_, err := svcCtx.DB.ExecCtx(ctx, `UPDATE t_source_webhook_event SET status = 5,claimed_by = NULL,lease_until = NULL,
error_message = ?,completed_at = CURRENT_TIMESTAMP(3) WHERE id = ? AND status = 2`, message, in.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fail source webhook event failed: %v", err)
	}
	return &core.RespBase{Base: okBase()}, nil
}

func listSourceWebhookEvents(ctx context.Context, svcCtx *svc.ServiceContext, in *core.SourceWebhookEventListReq) (*core.SourceWebhookEventListResp, error) {
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.GetPage())
	where, args := []string{"tenant_id = ?"}, []any{tenant}
	if in.GetTriggerId() > 0 {
		where, args = append(where, "trigger_id = ?"), append(args, in.TriggerId)
	}
	if in.GetStatus() != core.SourceWebhookEventStatus_SOURCE_WEBHOOK_EVENT_STATUS_UNKNOWN {
		where, args = append(where, "status = ?"), append(args, int64(in.Status))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_source_webhook_event WHERE "+whereSQL, args...); err != nil {
		return nil, status.Errorf(codes.Internal, "count source webhook events failed: %v", err)
	}
	queryArgs := append(append([]any{}, args...), cursor, limit+1)
	var rows []models.TSourceWebhookEvent
	if err := svcCtx.DB.QueryRowsCtx(ctx, &rows, `SELECT * FROM t_source_webhook_event WHERE `+whereSQL+` AND id > ? ORDER BY id ASC LIMIT ?`, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list source webhook events failed: %v", err)
	}
	hasNext := int64(len(rows)) > limit
	if hasNext {
		rows = rows[:limit]
	}
	data := make([]*core.SourceWebhookEvent, 0, len(rows))
	var next int64
	for index := range rows {
		data = append(data, mapSourceWebhookEvent(&rows[index]))
		next = rows[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.SourceWebhookEventListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
