package logic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"appforge/proto/core"
	"appforge/services/core/internal/svc"
	"appforge/services/core/models"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	buildCacheScopeTenant = int64(core.BuildCacheScope_BUILD_CACHE_SCOPE_TENANT_INTERMEDIATE)
	buildCacheActive      = int64(core.BuildCacheStatus_BUILD_CACHE_STATUS_ACTIVE)
	buildCacheInvalidated = int64(core.BuildCacheStatus_BUILD_CACHE_STATUS_INVALIDATED)
	buildCacheExpired     = int64(core.BuildCacheStatus_BUILD_CACHE_STATUS_EXPIRED)
)

const buildCacheSelect = `SELECT id, tenant_id, cache_scope, cache_key, toolchain_version,
build_protocol_version, input_digest, artifact_object_id, artifact_sha256, size_bytes, hit_count,
status, expires_at, last_hit_at, create_time, update_time FROM t_build_cache_entry`

const schedulerEventSelect = `SELECT id, tenant_id, app_id, task_id, node_code, pool_code,
event_type, reason_code, decision_json, create_time FROM t_build_scheduler_event`

const storageObjectSelect = `SELECT id, tenant_id, app_id, object_type, object_key, original_name,
content_type, size_bytes, sha256, status, storage_mode, owner_agent_id, create_by, create_time, update_time FROM t_storage_object`

func validSHA256(value string) bool {
	value = strings.TrimSpace(value)
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func effectiveBuildCacheKey(inputDigest, toolchainVersion string, protocolVersion int32) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", inputDigest, toolchainVersion, protocolVersion)))
	return hex.EncodeToString(digest[:])
}

func mapBuildCacheEntry(item *models.TBuildCacheEntry) *core.BuildCacheEntry {
	if item == nil {
		return nil
	}
	return &core.BuildCacheEntry{
		Id: item.Id, TenantId: item.TenantId, CacheScope: core.BuildCacheScope(item.CacheScope),
		CacheKey: item.CacheKey, ToolchainVersion: item.ToolchainVersion,
		BuildProtocolVersion: int32(item.BuildProtocolVersion), InputDigest: item.InputDigest,
		ArtifactObjectId: item.ArtifactObjectId, ArtifactSha256: item.ArtifactSha256,
		SizeBytes: item.SizeBytes, HitCount: item.HitCount, Status: core.BuildCacheStatus(item.Status),
		ExpiresAt: millis(item.ExpiresAt), LastHitAt: timeValue(item.LastHitAt),
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func mapSchedulerEvent(item *models.TBuildSchedulerEvent) *core.BuildSchedulerEvent {
	if item == nil {
		return nil
	}
	return &core.BuildSchedulerEvent{
		Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, TaskId: item.TaskId,
		NodeCode: stringValue(item.NodeCode), PoolCode: item.PoolCode,
		EventType: core.BuildSchedulerEventType(item.EventType), ReasonCode: stringValue(item.ReasonCode),
		DecisionJson: stringValue(item.DecisionJson), CreateTime: millis(item.CreateTime),
	}
}

func validateCacheWorkerRequest(taskID int64, nodeCode string, attempt, protocol int32, toolchain string) error {
	if taskID <= 0 || attempt <= 0 {
		return status.Error(codes.InvalidArgument, "task_id and builder_attempt must be greater than zero")
	}
	if err := validateBuilderRequest(nodeCode); err != nil {
		return err
	}
	if protocol <= 0 || strings.TrimSpace(toolchain) == "" || len(strings.TrimSpace(toolchain)) > 128 {
		return status.Error(codes.InvalidArgument, "toolchain_version and build_protocol_version are invalid")
	}
	return nil
}

func resolveBuildCache(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ResolveBuildCacheReq) (*core.BuildCacheResolutionResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateCacheWorkerRequest(in.TaskId, in.NodeCode, in.BuilderAttempt, in.BuildProtocolVersion, in.ToolchainVersion); err != nil {
		return nil, err
	}
	response := &core.BuildCacheResolutionResp{Base: okBase(), Data: &core.BuildCacheResolution{}}
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var task models.TBuildTask
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id = ? AND builder_id = ? AND builder_attempt = ?
AND status IN (?, ?, ?) AND lease_until > CURRENT_TIMESTAMP(3) FOR UPDATE`, in.TaskId, in.NodeCode,
			in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return status.Error(codes.NotFound, "build task is not owned by builder or lease has expired")
		}
		inputDigest := stringValue(task.CacheKey)
		if !validSHA256(inputDigest) {
			return nil
		}
		cacheKey := effectiveBuildCacheKey(inputDigest, strings.TrimSpace(in.ToolchainVersion), in.BuildProtocolVersion)
		var entry models.TBuildCacheEntry
		if err := session.QueryRowCtx(txCtx, &entry, buildCacheSelect+` WHERE tenant_id = ? AND cache_scope = ?
AND cache_key = ? AND status = ? AND expires_at > CURRENT_TIMESTAMP(3) FOR UPDATE`, task.TenantId,
			buildCacheScopeTenant, cacheKey, buildCacheActive); err != nil {
			if err == sql.ErrNoRows || err == sqlx.ErrNotFound {
				return nil
			}
			return err
		}
		if entry.InputDigest != inputDigest || entry.ToolchainVersion != strings.TrimSpace(in.ToolchainVersion) ||
			entry.BuildProtocolVersion != int64(in.BuildProtocolVersion) {
			return nil
		}
		var artifact models.TStorageObject
		if err := session.QueryRowCtx(txCtx, &artifact, storageObjectSelect+` WHERE id = ? AND tenant_id = ?
AND object_type = ? AND status IN (?, ?)`, entry.ArtifactObjectId, task.TenantId,
			int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_CACHE), storageStatusReady, storageStatusBound); err != nil {
			if _, updateErr := session.ExecCtx(txCtx, `UPDATE t_build_cache_entry SET status = ?, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, buildCacheInvalidated, entry.Id); updateErr != nil {
				return updateErr
			}
			return insertSchedulerEvent(txCtx, session, &task, in.NodeCode,
				core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_CACHE_INVALIDATED,
				"CACHE_OBJECT_MISSING", map[string]any{"cacheEntryId": entry.Id})
		}
		if stringValue(artifact.Sha256) != entry.ArtifactSha256 || artifact.SizeBytes != entry.SizeBytes {
			_, _ = session.ExecCtx(txCtx, `UPDATE t_build_cache_entry SET status = ?, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, buildCacheInvalidated, entry.Id)
			return insertSchedulerEvent(txCtx, session, &task, in.NodeCode,
				core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_CACHE_INVALIDATED,
				"CACHE_METADATA_MISMATCH", map[string]any{"cacheEntryId": entry.Id})
		}
		response.Data.Hit = true
		response.Data.Entry = mapBuildCacheEntry(&entry)
		response.Data.Artifact = mapStorageObject(&artifact)
		if !in.ConfirmHit {
			return nil
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_build_cache_entry SET hit_count = hit_count + 1,
last_hit_at = CURRENT_TIMESTAMP(3), update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, entry.Id); err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET cache_entry_id = ?, cache_hit = 1,
update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, entry.Id, task.Id); err != nil {
			return err
		}
		entry.HitCount++
		entry.LastHitAt = nullTime(time.Now())
		if err := insertSchedulerEvent(txCtx, session, &task, in.NodeCode,
			core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_CACHE_HIT, "VALIDATED_CACHE_HIT",
			map[string]any{"cacheEntryId": entry.Id, "artifactObjectId": entry.ArtifactObjectId}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

func publishBuildCache(ctx context.Context, svcCtx *svc.ServiceContext, in *core.PublishBuildCacheReq) (*core.BuildCacheEntryResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateCacheWorkerRequest(in.TaskId, in.NodeCode, in.BuilderAttempt, in.BuildProtocolVersion, in.ToolchainVersion); err != nil {
		return nil, err
	}
	if in.SizeBytes <= 0 || !validSHA256(in.ArtifactSha256) ||
		(in.ArtifactObjectId <= 0 && strings.TrimSpace(in.ArtifactObjectKey) == "") {
		return nil, status.Error(codes.InvalidArgument, "cache artifact metadata is invalid")
	}
	ttl := in.TtlSeconds
	if ttl <= 0 {
		ttl = 7 * 24 * 3600
	}
	if ttl > 90*24*3600 {
		return nil, status.Error(codes.InvalidArgument, "ttl_seconds exceeds 90 days")
	}
	var entry models.TBuildCacheEntry
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var task models.TBuildTask
		if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id = ? AND builder_id = ? AND builder_attempt = ?
AND status IN (?, ?, ?) AND lease_until > CURRENT_TIMESTAMP(3) FOR UPDATE`, in.TaskId, in.NodeCode,
			in.BuilderAttempt, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return status.Error(codes.NotFound, "build task is not owned by builder or lease has expired")
		}
		inputDigest := stringValue(task.CacheKey)
		if !validSHA256(inputDigest) {
			return status.Error(codes.FailedPrecondition, "build task has no valid input digest")
		}
		var artifact models.TStorageObject
		if in.ArtifactObjectId > 0 {
			if err := session.QueryRowCtx(txCtx, &artifact, storageObjectSelect+` WHERE id = ? AND tenant_id = ?
AND object_type = ? AND status IN (?, ?) FOR UPDATE`, in.ArtifactObjectId, task.TenantId,
				int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_CACHE), storageStatusReady, storageStatusBound); err != nil {
				return status.Error(codes.FailedPrecondition, "cache artifact is not ready")
			}
		} else {
			cacheArtifact := buildArtifact{
				ObjectKey: strings.TrimSpace(in.ArtifactObjectKey), OriginalName: "build-cache.apk",
				ContentType: "application/vnd.android.package-archive", Size: in.SizeBytes,
				SHA256: strings.TrimSpace(in.ArtifactSha256), ObjectType: int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_CACHE),
			}
			if err := validateBuildArtifact(task.TenantId, cacheArtifact, "build-cache"); err != nil {
				return err
			}
			artifactID, err := insertBuildArtifact(txCtx, session, task.TenantId, task.AppId, cacheArtifact)
			if err != nil {
				return err
			}
			artifact = models.TStorageObject{Id: artifactID, TenantId: task.TenantId, AppId: task.AppId,
				ObjectType: cacheArtifact.ObjectType, ObjectKey: cacheArtifact.ObjectKey,
				OriginalName: cacheArtifact.OriginalName, ContentType: cacheArtifact.ContentType,
				SizeBytes: cacheArtifact.Size, Sha256: nullString(cacheArtifact.SHA256), Status: storageStatusBound,
				StorageMode: int64(core.HybridArtifactMode_HYBRID_ARTIFACT_MODE_CONTROL_PLANE_STORAGE)}
		}
		if stringValue(artifact.Sha256) != strings.TrimSpace(in.ArtifactSha256) || artifact.SizeBytes != in.SizeBytes {
			return status.Error(codes.FailedPrecondition, "cache artifact metadata does not match storage object")
		}
		cacheKey := effectiveBuildCacheKey(inputDigest, strings.TrimSpace(in.ToolchainVersion), in.BuildProtocolVersion)
		_, err := session.ExecCtx(txCtx, `INSERT INTO t_build_cache_entry
(tenant_id, cache_scope, cache_key, toolchain_version, build_protocol_version, input_digest,
artifact_object_id, artifact_sha256, size_bytes, hit_count, status, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? SECOND))
ON DUPLICATE KEY UPDATE toolchain_version = VALUES(toolchain_version), build_protocol_version = VALUES(build_protocol_version),
input_digest = VALUES(input_digest), artifact_object_id = VALUES(artifact_object_id),
artifact_sha256 = VALUES(artifact_sha256), size_bytes = VALUES(size_bytes), status = VALUES(status),
expires_at = VALUES(expires_at), update_time = CURRENT_TIMESTAMP(3)`, task.TenantId, buildCacheScopeTenant,
			cacheKey, strings.TrimSpace(in.ToolchainVersion), in.BuildProtocolVersion, inputDigest,
			artifact.Id, strings.TrimSpace(in.ArtifactSha256), in.SizeBytes, buildCacheActive, ttl)
		if err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &entry, buildCacheSelect+` WHERE tenant_id = ? AND cache_scope = ? AND cache_key = ?`,
			task.TenantId, buildCacheScopeTenant, cacheKey)
	})
	if err != nil {
		return nil, err
	}
	return &core.BuildCacheEntryResp{Base: okBase(), Data: mapBuildCacheEntry(&entry)}, nil
}

func invalidateBuildCache(ctx context.Context, svcCtx *svc.ServiceContext, in *core.InvalidateBuildCacheReq) (*core.BuildCacheEntryResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "MANUAL_INVALIDATION"
	}
	if len(reason) > 500 {
		return nil, status.Error(codes.InvalidArgument, "reason is too long")
	}
	var entry models.TBuildCacheEntry
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &entry, buildCacheSelect+` WHERE id = ? FOR UPDATE`, in.Id); err != nil {
			return err
		}
		if in.TaskId > 0 {
			var task models.TBuildTask
			if err := session.QueryRowCtx(txCtx, &task, buildTaskSelect+` WHERE id = ? AND tenant_id = ?`, in.TaskId, entry.TenantId); err != nil {
				return status.Error(codes.InvalidArgument, "task does not belong to cache tenant")
			}
			if strings.TrimSpace(in.NodeCode) != "" && stringValue(task.BuilderId) != strings.TrimSpace(in.NodeCode) {
				return status.Error(codes.PermissionDenied, "builder does not own cache validation task")
			}
			if err := insertSchedulerEvent(txCtx, session, &task, in.NodeCode,
				core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_CACHE_INVALIDATED, reason,
				map[string]any{"cacheEntryId": entry.Id}); err != nil {
				return err
			}
			if _, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET cache_entry_id = 0, cache_hit = 0,
update_time = CURRENT_TIMESTAMP(3) WHERE id = ? AND cache_entry_id = ?`, task.Id, entry.Id); err != nil {
				return err
			}
		} else {
			tenant, tenantErr := tenantID(ctx)
			if tenantErr != nil || tenant != entry.TenantId {
				return status.Error(codes.NotFound, "build cache entry not found")
			}
		}
		if entry.Status == buildCacheActive {
			if _, err := session.ExecCtx(txCtx, `UPDATE t_build_cache_entry SET status = ?, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, buildCacheInvalidated, entry.Id); err != nil {
				return err
			}
			entry.Status = buildCacheInvalidated
		}
		return nil
	})
	if err != nil {
		return nil, notFoundOrInternal(err, "build cache entry")
	}
	return &core.BuildCacheEntryResp{Base: okBase(), Data: mapBuildCacheEntry(&entry)}, nil
}

func cleanupBuildCache(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CleanupBuildCacheReq) (*core.CleanupBuildCacheResp, error) {
	if in == nil {
		in = &core.CleanupBuildCacheReq{}
	}
	limit := int64(in.Limit)
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 || in.TargetFreeBytes < 0 {
		return nil, status.Error(codes.InvalidArgument, "cleanup limits are invalid")
	}
	result := &core.CleanupBuildCacheResult{}
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var expired []models.TBuildCacheEntry
		if err := session.QueryRowsCtx(txCtx, &expired, buildCacheSelect+` WHERE status = ? AND expires_at <= CURRENT_TIMESTAMP(3)
ORDER BY expires_at ASC, id ASC LIMIT ? FOR UPDATE SKIP LOCKED`, buildCacheActive, limit); err != nil {
			return err
		}
		expiredIDs := make(map[int64]struct{}, len(expired))
		for index := range expired {
			expiredIDs[expired[index].Id] = struct{}{}
		}
		candidates := expired
		remaining := limit - int64(len(candidates))
		if in.TargetFreeBytes > 0 && remaining > 0 {
			var lru []models.TBuildCacheEntry
			if err := session.QueryRowsCtx(txCtx, &lru, buildCacheSelect+` WHERE status = ? AND expires_at > CURRENT_TIMESTAMP(3)
ORDER BY COALESCE(last_hit_at, create_time) ASC, id ASC LIMIT ? FOR UPDATE SKIP LOCKED`, buildCacheActive, remaining); err != nil {
				return err
			}
			candidates = append(candidates, lru...)
		}
		for _, entry := range candidates {
			_, isExpired := expiredIDs[entry.Id]
			if in.TargetFreeBytes > 0 && result.ReclaimableBytes >= in.TargetFreeBytes && !isExpired {
				break
			}
			statusValue := buildCacheExpired
			if !isExpired {
				statusValue = buildCacheInvalidated
			}
			if _, err := session.ExecCtx(txCtx, `UPDATE t_build_cache_entry SET status = ?, update_time = CURRENT_TIMESTAMP(3) WHERE id = ? AND status = ?`,
				statusValue, entry.Id, buildCacheActive); err != nil {
				return err
			}
			result.InvalidatedCount++
			result.ReclaimableBytes += entry.SizeBytes
			var references int64
			if err := session.QueryRowCtx(txCtx, &references, `SELECT
(SELECT COUNT(1) FROM t_build_cache_entry WHERE artifact_object_id = ? AND status = ?) +
(SELECT COUNT(1) FROM t_build_task WHERE cache_entry_id = ? AND status IN (?, ?, ?, ?))`, entry.ArtifactObjectId,
				buildCacheActive, entry.Id, buildStatusBuilding, buildStatusSigning, buildStatusUploading, buildStatusSuccess); err != nil {
				return err
			}
			if references == 0 {
				storageResult, err := session.ExecCtx(txCtx, `UPDATE t_storage_object SET status = ?, update_time = CURRENT_TIMESTAMP(3)
WHERE id = ? AND object_type = ? AND status = ?`, storageStatusFailed, entry.ArtifactObjectId,
					int64(core.StorageObjectType_STORAGE_OBJECT_TYPE_BUILD_CACHE), storageStatusBound)
				if err != nil {
					return err
				}
				changed, _ := storageResult.RowsAffected()
				if changed == 1 {
					if err := adjustUsageInSession(txCtx, session, entry.TenantId, "storage.artifact_bytes", -entry.SizeBytes,
						"storage", entry.ArtifactObjectId, fmt.Sprintf("cache-storage-release:%d", entry.ArtifactObjectId),
						map[string]any{"reason": "build_cache_reclaimed"}); err != nil {
						return err
					}
				}
				result.ObjectIds = append(result.ObjectIds, entry.ArtifactObjectId)
			}
		}
		return nil
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cleanup build cache failed: %v", err)
	}
	return &core.CleanupBuildCacheResp{Base: okBase(), Data: result}, nil
}

func listBuildCacheEntries(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BuildCacheEntryListReq) (*core.BuildCacheEntryListResp, error) {
	if in == nil {
		in = &core.BuildCacheEntryListReq{}
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"id > ?", "tenant_id = ?"}
	args := []any{cursor, tenant}
	if in.CacheScope != core.BuildCacheScope_BUILD_CACHE_SCOPE_UNKNOWN {
		where = append(where, "cache_scope = ?")
		args = append(args, int64(in.CacheScope))
	}
	if in.Status != core.BuildCacheStatus_BUILD_CACHE_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(in.Status))
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		where = append(where, "(cache_key LIKE ? OR input_digest LIKE ? OR toolchain_version LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like, like)
	}
	filter := strings.Join(where, " AND ")
	var total int64
	countFilter := strings.Join(where[1:], " AND ")
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, `SELECT COUNT(1) FROM t_build_cache_entry WHERE `+countFilter, args[1:]...); err != nil {
		return nil, status.Errorf(codes.Internal, "count build cache entries failed: %v", err)
	}
	var items []models.TBuildCacheEntry
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, buildCacheSelect+` WHERE `+filter+` ORDER BY id ASC LIMIT ?`, append(args, limit+1)...); err != nil {
		return nil, status.Errorf(codes.Internal, "list build cache entries failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.BuildCacheEntry, 0, len(items))
	var next int64
	for index := range items {
		data = append(data, mapBuildCacheEntry(&items[index]))
		next = items[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.BuildCacheEntryListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func listBuildSchedulerEvents(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BuildSchedulerEventListReq) (*core.BuildSchedulerEventListResp, error) {
	if in == nil {
		in = &core.BuildSchedulerEventListReq{}
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"id > ?", "tenant_id = ?"}
	args := []any{cursor, tenant}
	for _, filter := range []struct {
		value int64
		field string
	}{{in.AppId, "app_id"}, {in.TaskId, "task_id"}} {
		if filter.value > 0 {
			where = append(where, filter.field+" = ?")
			args = append(args, filter.value)
		}
	}
	if value := strings.TrimSpace(in.NodeCode); value != "" {
		where = append(where, "node_code = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(in.PoolCode); value != "" {
		where = append(where, "pool_code = ?")
		args = append(args, value)
	}
	if in.EventType != core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_UNKNOWN {
		where = append(where, "event_type = ?")
		args = append(args, int64(in.EventType))
	}
	filter := strings.Join(where, " AND ")
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, `SELECT COUNT(1) FROM t_build_scheduler_event WHERE `+strings.Join(where[1:], " AND "), args[1:]...); err != nil {
		return nil, status.Errorf(codes.Internal, "count scheduler events failed: %v", err)
	}
	var items []models.TBuildSchedulerEvent
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, schedulerEventSelect+` WHERE `+filter+` ORDER BY id ASC LIMIT ?`, append(args, limit+1)...); err != nil {
		return nil, status.Errorf(codes.Internal, "list scheduler events failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.BuildSchedulerEvent, 0, len(items))
	var next int64
	for index := range items {
		data = append(data, mapSchedulerEvent(&items[index]))
		next = items[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.BuildSchedulerEventListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}
