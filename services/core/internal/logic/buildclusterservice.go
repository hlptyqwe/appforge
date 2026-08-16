package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
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
	builderNodeStatusOnline   = int64(core.BuilderNodeStatus_BUILDER_NODE_STATUS_ONLINE)
	builderNodeStatusOffline  = int64(core.BuilderNodeStatus_BUILDER_NODE_STATUS_OFFLINE)
	builderNodeStatusIsolated = int64(core.BuilderNodeStatus_BUILDER_NODE_STATUS_ISOLATED)
	builderDrainAccepting     = int64(core.BuilderDrainStatus_BUILDER_DRAIN_STATUS_ACCEPTING)
	builderDrainDraining      = int64(core.BuilderDrainStatus_BUILDER_DRAIN_STATUS_DRAINING)
	buildPolicyEnabled        = int64(core.BuildPolicyStatus_BUILD_POLICY_STATUS_ENABLED)
	buildSlotActive           = int64(core.BuildSlotLeaseStatus_BUILD_SLOT_LEASE_STATUS_ACTIVE)
	buildSlotReleased         = int64(core.BuildSlotLeaseStatus_BUILD_SLOT_LEASE_STATUS_RELEASED)
	buildSlotExpired          = int64(core.BuildSlotLeaseStatus_BUILD_SLOT_LEASE_STATUS_EXPIRED)
	buildSlotCancelled        = int64(core.BuildSlotLeaseStatus_BUILD_SLOT_LEASE_STATUS_CANCELLED)
	defaultBuildPool          = "default"
	defaultGlobalConcurrency  = int64(10)
	defaultTenantConcurrency  = int64(2)
	defaultAppConcurrency     = int64(1)
	builderRecoveryHeartbeat  = 90 * time.Second
	builderMinimumDiskFree    = int64(512 * 1024 * 1024)
)

var buildPoolCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)

const builderNodeSelect = `SELECT id, node_code, pool_code, endpoint, status, drain_status,
max_concurrency, running_count, cpu_capacity, memory_capacity, disk_capacity, disk_free,
toolchain_version, build_protocol_version, capability_json, consecutive_failures,
last_error_message, last_heartbeat_at, create_time, update_time FROM t_builder_node`

const buildPolicySelect = `SELECT id, tenant_id, app_id, pool_code, max_concurrency,
fair_weight, max_priority, status, create_by, create_time, update_time FROM t_build_concurrency_policy`

func normalizedBuildPool(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultBuildPool, nil
	}
	if !buildPoolCodePattern.MatchString(value) {
		return "", status.Error(codes.InvalidArgument, "pool_code must use 2-64 lowercase letters, digits, underscores or hyphens")
	}
	return value, nil
}

func normalizedBuilderNodeCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !buildPoolCodePattern.MatchString(value) {
		return "", status.Error(codes.InvalidArgument, "node_code must use 2-64 lowercase letters, digits, underscores or hyphens")
	}
	return value, nil
}

func validateCapabilityJSON(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "{}", nil
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(value), &object); err != nil {
		return "", status.Error(codes.InvalidArgument, "capability_json must be a JSON object")
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return "", status.Error(codes.InvalidArgument, "capability_json cannot be normalized")
	}
	return string(normalized), nil
}

func builderNodeSupportsRemoteSigning(node *models.TBuilderNode) bool {
	if node == nil {
		return false
	}
	var capabilities map[string]any
	if err := json.Unmarshal([]byte(stringValue(node.CapabilityJson)), &capabilities); err != nil {
		return false
	}
	enabled, ok := capabilities["remoteSigning"].(bool)
	return ok && enabled
}

func taskRequiresRemoteSigning(ctx context.Context, session sqlx.Session, task *models.TBuildTask) (bool, error) {
	if task == nil || task.SigningConfigId <= 0 {
		return false, nil
	}
	var signing struct {
		KeystoreObjectID int64          `db:"keystore_object_id"`
		SecretRef        sql.NullString `db:"secret_ref"`
	}
	if err := session.QueryRowCtx(ctx, &signing, `SELECT keystore_object_id, secret_ref
FROM t_app_signing_config WHERE id = ? AND tenant_id = ? AND app_id = ? LIMIT 1`,
		task.SigningConfigId, task.TenantId, task.AppId); err != nil {
		return false, err
	}
	return signing.KeystoreObjectID == 0 && strings.TrimSpace(stringValue(signing.SecretRef)) != "", nil
}

func mapBuilderNode(item *models.TBuilderNode) *core.BuilderNode {
	if item == nil {
		return nil
	}
	return &core.BuilderNode{
		Id: item.Id, NodeCode: item.NodeCode, PoolCode: item.PoolCode,
		Endpoint: stringValue(item.Endpoint), Status: core.BuilderNodeStatus(item.Status),
		DrainStatus: core.BuilderDrainStatus(item.DrainStatus), MaxConcurrency: int32(item.MaxConcurrency),
		RunningCount: int32(item.RunningCount), CpuCapacity: int32(item.CpuCapacity),
		MemoryCapacity: item.MemoryCapacity, DiskCapacity: item.DiskCapacity, DiskFree: item.DiskFree,
		ToolchainVersion: item.ToolchainVersion, BuildProtocolVersion: int32(item.BuildProtocolVersion),
		CapabilityJson: stringValue(item.CapabilityJson), ConsecutiveFailures: int32(item.ConsecutiveFailures),
		LastErrorMessage: stringValue(item.LastErrorMessage), LastHeartbeatAt: millis(item.LastHeartbeatAt),
		CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func mapBuildPolicy(item *models.TBuildConcurrencyPolicy) *core.BuildConcurrencyPolicy {
	if item == nil {
		return nil
	}
	return &core.BuildConcurrencyPolicy{
		Id: item.Id, TenantId: item.TenantId, AppId: item.AppId, PoolCode: item.PoolCode,
		MaxConcurrency: int32(item.MaxConcurrency), FairWeight: int32(item.FairWeight),
		MaxPriority: int32(item.MaxPriority), Status: core.BuildPolicyStatus(item.Status),
		CreateBy: item.CreateBy, CreateTime: millis(item.CreateTime), UpdateTime: millis(item.UpdateTime),
	}
}

func registerBuilderNode(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RegisterBuilderNodeReq) (*core.BuilderNodeResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	nodeCode, err := normalizedBuilderNodeCode(in.NodeCode)
	if err != nil {
		return nil, err
	}
	poolCode, err := normalizedBuildPool(in.PoolCode)
	if err != nil {
		return nil, err
	}
	if err := requireOptionalText(in.Endpoint, "endpoint", 255); err != nil {
		return nil, err
	}
	if in.MaxConcurrency <= 0 || in.MaxConcurrency > 64 {
		return nil, status.Error(codes.InvalidArgument, "max_concurrency must be between 1 and 64")
	}
	if err := requireText(in.ToolchainVersion, "toolchain_version", 128); err != nil {
		return nil, err
	}
	if in.BuildProtocolVersion <= 0 {
		return nil, status.Error(codes.InvalidArgument, "build_protocol_version must be greater than zero")
	}
	if in.CpuCapacity < 0 || in.MemoryCapacity < 0 || in.DiskCapacity < 0 || in.DiskFree < 0 || (in.DiskCapacity > 0 && in.DiskFree > in.DiskCapacity) {
		return nil, status.Error(codes.InvalidArgument, "builder capacity values are invalid")
	}
	capabilityJSON, err := validateCapabilityJSON(in.CapabilityJson)
	if err != nil {
		return nil, err
	}
	result, err := svcCtx.DB.ExecCtx(ctx, `INSERT INTO t_builder_node
(node_code, pool_code, endpoint, status, drain_status, max_concurrency, running_count,
cpu_capacity, memory_capacity, disk_capacity, disk_free, toolchain_version, build_protocol_version,
capability_json, last_heartbeat_at) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP(3))
ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id), pool_code = VALUES(pool_code), endpoint = VALUES(endpoint),
status = IF(status = 3, status, VALUES(status)), max_concurrency = VALUES(max_concurrency),
cpu_capacity = VALUES(cpu_capacity), memory_capacity = VALUES(memory_capacity),
disk_capacity = VALUES(disk_capacity), disk_free = VALUES(disk_free),
toolchain_version = VALUES(toolchain_version), build_protocol_version = VALUES(build_protocol_version),
capability_json = VALUES(capability_json), last_heartbeat_at = VALUES(last_heartbeat_at),
update_time = CURRENT_TIMESTAMP(3)`,

		nodeCode, poolCode, strings.TrimSpace(in.Endpoint), builderNodeStatusOnline, builderDrainAccepting,
		in.MaxConcurrency, in.CpuCapacity, in.MemoryCapacity, in.DiskCapacity, in.DiskFree,
		strings.TrimSpace(in.ToolchainVersion), in.BuildProtocolVersion, capabilityJSON)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register builder node failed: %v", err)
	}
	itemID, err := result.LastInsertId()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read builder node id failed: %v", err)
	}
	var item models.TBuilderNode
	if err := svcCtx.DB.QueryRowCtx(ctx, &item, builderNodeSelect+` WHERE id = ?`, itemID); err != nil {
		return nil, status.Errorf(codes.Internal, "load registered builder node failed: %v", err)
	}
	return &core.BuilderNodeResp{Base: okBase(), Data: mapBuilderNode(&item)}, nil
}

func heartbeatBuilderNode(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BuilderNodeHeartbeatReq) (*core.BuilderNodeResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	nodeCode, err := normalizedBuilderNodeCode(in.NodeCode)
	if err != nil {
		return nil, err
	}
	if in.RunningCount < 0 || in.DiskFree < 0 {
		return nil, status.Error(codes.InvalidArgument, "heartbeat capacity values are invalid")
	}
	if err := requireOptionalText(in.LastErrorMessage, "last_error_message", 1000); err != nil {
		return nil, err
	}
	var item models.TBuilderNode
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if findErr := session.QueryRowCtx(txCtx, &item, builderNodeSelect+` WHERE node_code = ? FOR UPDATE`, nodeCode); findErr != nil {
			return findErr
		}
		if int64(in.RunningCount) > item.MaxConcurrency || (item.DiskCapacity > 0 && in.DiskFree > item.DiskCapacity) {
			return status.Error(codes.InvalidArgument, "heartbeat exceeds registered node capacity")
		}
		failureCount := item.ConsecutiveFailures
		lastError := strings.TrimSpace(in.LastErrorMessage)
		if lastError == "" {
			failureCount = 0
		} else {
			failureCount++
		}
		statusValue := item.Status
		if statusValue == builderNodeStatusOffline {
			statusValue = builderNodeStatusOnline
		}
		lowDisk := in.DiskFree > 0 && (in.DiskFree < 512*1024*1024 || (item.DiskCapacity > 0 && in.DiskFree*100 < item.DiskCapacity*2))
		if failureCount >= 3 || lowDisk {
			statusValue = builderNodeStatusIsolated
		}
		_, updateErr := session.ExecCtx(txCtx, `UPDATE t_builder_node SET status = ?, running_count = ?, disk_free = ?,
consecutive_failures = ?, last_error_message = NULLIF(?, ''), last_heartbeat_at = CURRENT_TIMESTAMP(3),
update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, statusValue, in.RunningCount, in.DiskFree,
			failureCount, lastError, item.Id)
		if updateErr != nil {
			return updateErr
		}
		return session.QueryRowCtx(txCtx, &item, builderNodeSelect+` WHERE id = ?`, item.Id)
	})
	if err != nil {
		if err == sql.ErrNoRows || err == sqlx.ErrNotFound {
			return nil, status.Error(codes.NotFound, "builder node is not registered")
		}
		return nil, err
	}
	return &core.BuilderNodeResp{Base: okBase(), Data: mapBuilderNode(&item)}, nil
}

func getBuilderNode(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BuilderNodeIdReq) (*core.BuilderNodeResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	var item models.TBuilderNode
	if err := svcCtx.DB.QueryRowCtx(ctx, &item, builderNodeSelect+` WHERE id = ?`, in.Id); err != nil {
		return nil, notFoundOrInternal(err, "builder node")
	}
	return &core.BuilderNodeResp{Base: okBase(), Data: mapBuilderNode(&item)}, nil
}

func listBuilderNodes(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BuilderNodeListReq) (*core.BuilderNodeListResp, error) {
	if in == nil {
		in = &core.BuilderNodeListReq{}
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"id > ?"}
	args := []any{cursor}
	if strings.TrimSpace(in.PoolCode) != "" {
		poolCode, err := normalizedBuildPool(in.PoolCode)
		if err != nil {
			return nil, err
		}
		where = append(where, "pool_code = ?")
		args = append(args, poolCode)
	}
	if in.Status != core.BuilderNodeStatus_BUILDER_NODE_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(in.Status))
	}
	if in.DrainStatus != core.BuilderDrainStatus_BUILDER_DRAIN_STATUS_UNKNOWN {
		where = append(where, "drain_status = ?")
		args = append(args, int64(in.DrainStatus))
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		where = append(where, "(node_code LIKE ? OR endpoint LIKE ?)")
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	filter := strings.Join(where, " AND ")
	var total int64
	countWhere := strings.TrimPrefix(filter, "id > ? AND ")
	countArgs := args[1:]
	if countWhere == filter {
		countWhere = "1 = 1"
		countArgs = nil
	}
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM t_builder_node WHERE "+countWhere, countArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list builder nodes count failed: %v", err)
	}
	queryArgs := append(args, limit+1)
	var items []models.TBuilderNode
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, builderNodeSelect+` WHERE `+filter+` ORDER BY id ASC LIMIT ?`, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list builder nodes failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.BuilderNode, 0, len(items))
	var next int64
	for index := range items {
		data = append(data, mapBuilderNode(&items[index]))
		next = items[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.BuilderNodeListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

func drainBuilderNode(ctx context.Context, svcCtx *svc.ServiceContext, in *core.DrainBuilderNodeReq) (*core.BuilderNodeResp, error) {
	if in == nil || (in.Id <= 0 && strings.TrimSpace(in.NodeCode) == "") {
		return nil, status.Error(codes.InvalidArgument, "builder node id or node_code is required")
	}
	if in.DrainStatus != core.BuilderDrainStatus_BUILDER_DRAIN_STATUS_ACCEPTING && in.DrainStatus != core.BuilderDrainStatus_BUILDER_DRAIN_STATUS_DRAINING {
		return nil, status.Error(codes.InvalidArgument, "drain_status is invalid")
	}
	var item models.TBuilderNode
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		where := "id = ?"
		arg := any(in.Id)
		if in.Id <= 0 {
			where = "node_code = ?"
			arg = strings.TrimSpace(in.NodeCode)
		}
		if err := session.QueryRowCtx(txCtx, &item, builderNodeSelect+` WHERE `+where+` FOR UPDATE`, arg); err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_builder_node SET drain_status = ?, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, int64(in.DrainStatus), item.Id); err != nil {
			return err
		}
		decision, _ := json.Marshal(map[string]any{"drainStatus": in.DrainStatus.String(), "runningCount": item.RunningCount})
		if _, err := session.ExecCtx(txCtx, `INSERT INTO t_build_scheduler_event
(node_code, pool_code, event_type, reason_code, decision_json) VALUES (?, ?, ?, ?, ?)`, item.NodeCode,
			item.PoolCode, int64(core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_DRAINED), "NODE_DRAIN_STATUS_CHANGED", string(decision)); err != nil {
			return err
		}
		item.DrainStatus = int64(in.DrainStatus)
		return nil
	})
	if err != nil {
		return nil, notFoundOrInternal(err, "builder node")
	}
	return &core.BuilderNodeResp{Base: okBase(), Data: mapBuilderNode(&item)}, nil
}

func builderNodeRecoveryError(item *models.TBuilderNode, now time.Time) error {
	if item == nil {
		return status.Error(codes.InvalidArgument, "builder node is required")
	}
	if item.Status != builderNodeStatusIsolated {
		return status.Error(codes.FailedPrecondition, "builder node is not isolated")
	}
	if item.LastHeartbeatAt.IsZero() || now.Sub(item.LastHeartbeatAt) < 0 || now.Sub(item.LastHeartbeatAt) > builderRecoveryHeartbeat {
		return status.Error(codes.FailedPrecondition, "builder node heartbeat is stale")
	}
	if item.ConsecutiveFailures != 0 {
		return status.Error(codes.FailedPrecondition, "builder node still reports consecutive failures")
	}
	if item.DiskCapacity <= 0 || item.DiskFree < builderMinimumDiskFree || item.DiskFree*100 < item.DiskCapacity*2 {
		return status.Error(codes.FailedPrecondition, "builder node disk capacity has not recovered")
	}
	return nil
}

func recoverBuilderNode(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RecoverBuilderNodeReq) (*core.BuilderNodeResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	reason := strings.TrimSpace(in.Reason)
	if len([]rune(reason)) < 2 {
		return nil, status.Error(codes.InvalidArgument, "reason must contain at least 2 characters")
	}
	if err := requireText(reason, "reason", 200); err != nil {
		return nil, err
	}

	var item models.TBuilderNode
	err := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, builderNodeSelect+` WHERE id = ? FOR UPDATE`, in.Id); err != nil {
			return err
		}
		if err := builderNodeRecoveryError(&item, time.Now()); err != nil {
			return err
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_builder_node SET status = ?, consecutive_failures = 0,
last_error_message = NULL, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, builderNodeStatusOnline, item.Id); err != nil {
			return err
		}
		decision, _ := json.Marshal(map[string]any{
			"diskCapacity": item.DiskCapacity,
			"diskFree":     item.DiskFree,
			"reason":       reason,
		})
		if _, err := session.ExecCtx(txCtx, `INSERT INTO t_build_scheduler_event
(node_code, pool_code, event_type, reason_code, decision_json) VALUES (?, ?, ?, ?, ?)`, item.NodeCode,
			item.PoolCode, int64(core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_RECOVERED),
			"NODE_ISOLATION_RECOVERED", string(decision)); err != nil {
			return err
		}
		return session.QueryRowCtx(txCtx, &item, builderNodeSelect+` WHERE id = ?`, item.Id)
	})
	if err != nil {
		if status.Code(err) != codes.Unknown {
			return nil, err
		}
		return nil, notFoundOrInternal(err, "builder node")
	}
	return &core.BuilderNodeResp{Base: okBase(), Data: mapBuilderNode(&item)}, nil
}

func upsertBuildPolicy(ctx context.Context, svcCtx *svc.ServiceContext, in *core.UpsertBuildConcurrencyPolicyReq) (*core.BuildConcurrencyPolicyResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	requestTenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	in.TenantId = requestTenant
	poolCode, err := normalizedBuildPool(in.PoolCode)
	if err != nil {
		return nil, err
	}
	if in.TenantId < 0 || in.AppId < 0 || (in.TenantId == 0 && in.AppId > 0) {
		return nil, status.Error(codes.InvalidArgument, "policy scope is invalid")
	}
	if in.MaxConcurrency <= 0 || in.MaxConcurrency > 10000 || in.FairWeight <= 0 || in.FairWeight > 10000 || in.MaxPriority < 0 || in.MaxPriority > 10000 {
		return nil, status.Error(codes.InvalidArgument, "policy limits are invalid")
	}
	if in.Status != core.BuildPolicyStatus_BUILD_POLICY_STATUS_ENABLED && in.Status != core.BuildPolicyStatus_BUILD_POLICY_STATUS_DISABLED {
		return nil, status.Error(codes.InvalidArgument, "policy status is invalid")
	}
	if in.AppId > 0 {
		app, findErr := svcCtx.ApplicationModel.FindOne(ctx, in.AppId)
		if findErr != nil || app.TenantId != in.TenantId {
			return nil, status.Error(codes.InvalidArgument, "policy application does not belong to tenant")
		}
	}
	var item models.TBuildConcurrencyPolicy
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if in.Id > 0 {
			if err := session.QueryRowCtx(txCtx, &item, buildPolicySelect+` WHERE id = ? FOR UPDATE`, in.Id); err != nil {
				return err
			}
			_, err := session.ExecCtx(txCtx, `UPDATE t_build_concurrency_policy SET tenant_id = ?, app_id = ?, pool_code = ?,
max_concurrency = ?, fair_weight = ?, max_priority = ?, status = ?, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`,
				in.TenantId, in.AppId, poolCode, in.MaxConcurrency, in.FairWeight, in.MaxPriority, int64(in.Status), in.Id)
			if err != nil {
				return err
			}
			item.Id = in.Id
		} else {
			_, err := session.ExecCtx(txCtx, `INSERT INTO t_build_concurrency_policy
(tenant_id, app_id, pool_code, max_concurrency, fair_weight, max_priority, status, create_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE max_concurrency = VALUES(max_concurrency),
fair_weight = VALUES(fair_weight), max_priority = VALUES(max_priority), status = VALUES(status),
update_time = CURRENT_TIMESTAMP(3)`, in.TenantId, in.AppId, poolCode, in.MaxConcurrency,
				in.FairWeight, in.MaxPriority, int64(in.Status), actorID(ctx))
			if err != nil {
				return err
			}
		}
		return session.QueryRowCtx(txCtx, &item, buildPolicySelect+` WHERE tenant_id = ? AND app_id = ? AND pool_code = ?`, in.TenantId, in.AppId, poolCode)
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "upsert build policy failed: %v", err)
	}
	return &core.BuildConcurrencyPolicyResp{Base: okBase(), Data: mapBuildPolicy(&item)}, nil
}

func listBuildPolicies(ctx context.Context, svcCtx *svc.ServiceContext, in *core.BuildConcurrencyPolicyListReq) (*core.BuildConcurrencyPolicyListResp, error) {
	if in == nil {
		in = &core.BuildConcurrencyPolicyListReq{}
	}
	requestTenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	cursor, limit := pageValues(in.Page)
	where := []string{"id > ?", "tenant_id IN (0, ?)"}
	args := []any{cursor, requestTenant}
	if in.AppId > 0 {
		where = append(where, "app_id = ?")
		args = append(args, in.AppId)
	}
	if strings.TrimSpace(in.PoolCode) != "" {
		poolCode, err := normalizedBuildPool(in.PoolCode)
		if err != nil {
			return nil, err
		}
		where = append(where, "pool_code = ?")
		args = append(args, poolCode)
	}
	if in.Status != core.BuildPolicyStatus_BUILD_POLICY_STATUS_UNKNOWN {
		where = append(where, "status = ?")
		args = append(args, int64(in.Status))
	}
	filter := strings.Join(where, " AND ")
	var items []models.TBuildConcurrencyPolicy
	queryArgs := append(args, limit+1)
	if err := svcCtx.DB.QueryRowsCtx(ctx, &items, buildPolicySelect+` WHERE `+filter+` ORDER BY id ASC LIMIT ?`, queryArgs...); err != nil {
		return nil, status.Errorf(codes.Internal, "list build policies failed: %v", err)
	}
	var total int64
	if err := svcCtx.DB.QueryRowCtx(ctx, &total, `SELECT COUNT(1) FROM t_build_concurrency_policy WHERE `+
		strings.Join(where[1:], " AND "), args[1:]...); err != nil {
		return nil, status.Errorf(codes.Internal, "count build policies failed: %v", err)
	}
	hasNext := int64(len(items)) > limit
	if hasNext {
		items = items[:limit]
	}
	data := make([]*core.BuildConcurrencyPolicy, 0, len(items))
	var next int64
	for index := range items {
		data = append(data, mapBuildPolicy(&items[index]))
		next = items[index].Id
	}
	if !hasNext {
		next = 0
	}
	return &core.BuildConcurrencyPolicyListResp{Base: baseWithTotal(total, hasNext, next), Data: data}, nil
}

type schedulerTenantCandidate struct {
	TenantID      int64 `db:"tenant_id"`
	VirtualFinish int64 `db:"virtual_finish"`
}

func schedulerPolicy(ctx context.Context, session sqlx.Session, tenantID, appID int64, poolCode string) (maxConcurrency, fairWeight, maxPriority int64, err error) {
	defaults := defaultAppConcurrency
	if tenantID == 0 {
		defaults = defaultGlobalConcurrency
	} else if appID == 0 {
		defaults = defaultTenantConcurrency
	}
	maxConcurrency, fairWeight, maxPriority = defaults, 100, 100
	var item struct {
		MaxConcurrency int64 `db:"max_concurrency"`
		FairWeight     int64 `db:"fair_weight"`
		MaxPriority    int64 `db:"max_priority"`
	}
	queryErr := session.QueryRowCtx(ctx, &item, `SELECT max_concurrency, fair_weight, max_priority
FROM t_build_concurrency_policy WHERE tenant_id = ? AND app_id = ? AND pool_code = ? AND status = ? LIMIT 1`,
		tenantID, appID, poolCode, buildPolicyEnabled)
	if queryErr == nil {
		return item.MaxConcurrency, item.FairWeight, item.MaxPriority, nil
	}
	if queryErr == sql.ErrNoRows || queryErr == sqlx.ErrNotFound {
		return maxConcurrency, fairWeight, maxPriority, nil
	}
	return 0, 0, 0, queryErr
}

func activeSlotCount(ctx context.Context, session sqlx.Session, poolCode string, tenantID, appID int64) (int64, error) {
	query := `SELECT COUNT(1) FROM t_build_slot_lease WHERE pool_code = ? AND status = ? AND lease_until > CURRENT_TIMESTAMP(3)`
	args := []any{poolCode, buildSlotActive}
	if tenantID > 0 {
		query += " AND tenant_id = ?"
		args = append(args, tenantID)
	}
	if appID > 0 {
		query += " AND app_id = ?"
		args = append(args, appID)
	}
	var count int64
	return count, session.QueryRowCtx(ctx, &count, query, args...)
}

func insertSchedulerEvent(ctx context.Context, session sqlx.Session, item *models.TBuildTask, nodeCode string, eventType core.BuildSchedulerEventType, reason string, decision any) error {
	encoded, _ := json.Marshal(decision)
	_, err := session.ExecCtx(ctx, `INSERT INTO t_build_scheduler_event
(tenant_id, app_id, task_id, node_code, pool_code, event_type, reason_code, decision_json)
VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?)`, item.TenantId, item.AppId, item.Id,
		nodeCode, item.PoolCode, int64(eventType), reason, string(encoded))
	return err
}

func claimScheduledTask(ctx context.Context, svcCtx *svc.ServiceContext, in *core.ClaimScheduledBuildTaskReq) (*core.BuildTaskResp, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	nodeCode, err := normalizedBuilderNodeCode(in.NodeCode)
	if err != nil {
		return nil, err
	}
	seconds := leaseSeconds(in.LeaseSeconds)
	var claimed *models.TBuildTask
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		var node models.TBuilderNode
		if err := session.QueryRowCtx(txCtx, &node, builderNodeSelect+` WHERE node_code = ? FOR UPDATE`, nodeCode); err != nil {
			return status.Error(codes.FailedPrecondition, "builder node is not registered")
		}
		if node.Status != builderNodeStatusOnline || node.DrainStatus != builderDrainAccepting || time.Since(node.LastHeartbeatAt) > 90*time.Second {
			if time.Since(node.LastHeartbeatAt) > 90*time.Second && node.Status != builderNodeStatusIsolated {
				_, _ = session.ExecCtx(txCtx, `UPDATE t_builder_node SET status = ?, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, builderNodeStatusOffline, node.Id)
			}
			return nil
		}
		if node.BuildProtocolVersion < 1 || strings.TrimSpace(node.ToolchainVersion) == "" {
			return status.Error(codes.FailedPrecondition, "builder node capability is incomplete")
		}
		supportsRemoteSigning := builderNodeSupportsRemoteSigning(&node)
		_, _ = session.ExecCtx(txCtx, `UPDATE t_build_slot_lease SET status = ?, released_at = CURRENT_TIMESTAMP(3),
update_time = CURRENT_TIMESTAMP(3) WHERE status = ? AND lease_until <= CURRENT_TIMESTAMP(3)`, buildSlotExpired, buildSlotActive)
		nodeCount, err := activeSlotCount(txCtx, session, node.PoolCode, 0, 0)
		if err != nil {
			return err
		}
		var nodeActive int64
		if err := session.QueryRowCtx(txCtx, &nodeActive, `SELECT COUNT(1) FROM t_build_slot_lease
WHERE node_code = ? AND status = ? AND lease_until > CURRENT_TIMESTAMP(3)`, node.NodeCode, buildSlotActive); err != nil {
			return err
		}
		if nodeActive >= node.MaxConcurrency {
			return nil
		}
		globalMax, _, _, err := schedulerPolicy(txCtx, session, 0, 0, node.PoolCode)
		if err != nil {
			return err
		}
		if nodeCount >= globalMax {
			return nil
		}
		if _, err := session.ExecCtx(txCtx, `INSERT INTO t_build_fair_queue (tenant_id, pool_code)
SELECT DISTINCT tenant_id, pool_code FROM t_build_task WHERE pool_code = ? AND
(status = ? OR (status IN (?, ?, ?) AND (lease_until IS NULL OR lease_until < CURRENT_TIMESTAMP(3))))
ON DUPLICATE KEY UPDATE tenant_id = VALUES(tenant_id)`, node.PoolCode, buildStatusPending,
			buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return err
		}
		var tenants []schedulerTenantCandidate
		if err := session.QueryRowsCtx(txCtx, &tenants, `SELECT f.tenant_id, f.virtual_finish
FROM t_build_fair_queue f WHERE f.pool_code = ? AND EXISTS (
SELECT 1 FROM t_build_task t WHERE t.tenant_id = f.tenant_id AND t.pool_code = f.pool_code AND
(t.status = ? OR (t.status IN (?, ?, ?) AND (t.lease_until IS NULL OR t.lease_until < CURRENT_TIMESTAMP(3)))))
ORDER BY f.virtual_finish ASC, f.last_dispatched_at ASC, f.tenant_id ASC LIMIT 64`, node.PoolCode,
			buildStatusPending, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
			return err
		}
		for _, tenantCandidate := range tenants {
			tenantMax, fairWeight, _, policyErr := schedulerPolicy(txCtx, session, tenantCandidate.TenantID, 0, node.PoolCode)
			if policyErr != nil {
				return policyErr
			}
			subscription, entitlement, _, billingErr := loadTenantBilling(txCtx, session, tenantCandidate.TenantID, true)
			if billingErr != nil || !subscriptionAllowsConsumption(subscription, billingNow()) ||
				entitlement.Status != entitlementActive || !billingNow().Before(entitlement.ValidUntil) {
				continue
			}
			if entitlement.MaxBuildConcurrency == 0 {
				continue
			}
			if entitlement.MaxBuildConcurrency > 0 && (tenantMax < 0 || entitlement.MaxBuildConcurrency < tenantMax) {
				tenantMax = entitlement.MaxBuildConcurrency
			}
			tenantCount, countErr := activeSlotCount(txCtx, session, node.PoolCode, tenantCandidate.TenantID, 0)
			if countErr != nil {
				return countErr
			}
			if tenantCount >= tenantMax {
				continue
			}
			var tasks []models.TBuildTask
			if err := session.QueryRowsCtx(txCtx, &tasks, buildTaskSelect+` WHERE tenant_id = ? AND pool_code = ? AND
(status = ? OR (status IN (?, ?, ?) AND (lease_until IS NULL OR lease_until < CURRENT_TIMESTAMP(3))))
ORDER BY priority DESC, queued_at ASC, id ASC LIMIT 32 FOR UPDATE SKIP LOCKED`, tenantCandidate.TenantID,
				node.PoolCode, buildStatusPending, buildStatusBuilding, buildStatusSigning, buildStatusUploading); err != nil {
				return err
			}
			for index := range tasks {
				task := &tasks[index]
				requiresRemoteSigning, capabilityErr := taskRequiresRemoteSigning(txCtx, session, task)
				if capabilityErr != nil {
					return capabilityErr
				}
				if requiresRemoteSigning && !supportsRemoteSigning {
					continue
				}
				appMax, _, _, policyErr := schedulerPolicy(txCtx, session, task.TenantId, task.AppId, node.PoolCode)
				if policyErr != nil {
					return policyErr
				}
				appCount, countErr := activeSlotCount(txCtx, session, node.PoolCode, task.TenantId, task.AppId)
				if countErr != nil {
					return countErr
				}
				if appCount >= appMax {
					continue
				}
				wasRecovery := task.Status != buildStatusPending
				newAttempt := task.BuilderAttempt + 1
				result, updateErr := session.ExecCtx(txCtx, `UPDATE t_build_task SET status = ?, builder_id = ?,
builder_attempt = ?, start_time = COALESCE(start_time, CURRENT_TIMESTAMP(3)),
lease_until = DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? SECOND), update_time = CURRENT_TIMESTAMP(3)
WHERE id = ? AND builder_attempt = ?`, buildStatusBuilding, node.NodeCode, newAttempt, seconds, task.Id, task.BuilderAttempt)
				if updateErr != nil {
					return updateErr
				}
				affected, _ := result.RowsAffected()
				if affected != 1 {
					continue
				}
				if _, insertErr := session.ExecCtx(txCtx, `INSERT INTO t_build_slot_lease
(task_id, tenant_id, app_id, node_code, pool_code, builder_attempt, status, lease_until)
VALUES (?, ?, ?, ?, ?, ?, ?, DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? SECOND))`, task.Id,
					task.TenantId, task.AppId, node.NodeCode, node.PoolCode, newAttempt, buildSlotActive, seconds); insertErr != nil {
					return insertErr
				}
				increment := int64(1)
				if fairWeight > 0 {
					increment = 100000 / fairWeight
					if increment < 1 {
						increment = 1
					}
				}
				if _, updateErr := session.ExecCtx(txCtx, `UPDATE t_build_fair_queue SET virtual_finish = virtual_finish + ?,
dispatch_count = dispatch_count + 1, last_dispatched_at = CURRENT_TIMESTAMP(3), update_time = CURRENT_TIMESTAMP(3)
WHERE tenant_id = ? AND pool_code = ?`, increment, task.TenantId, node.PoolCode); updateErr != nil {
					return updateErr
				}
				task.Status = buildStatusBuilding
				task.BuilderId = nullString(node.NodeCode)
				task.BuilderAttempt = newAttempt
				eventType := core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_CLAIMED
				reason := "FAIR_QUEUE_CLAIM"
				if wasRecovery {
					eventType = core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_RECOVERED
					reason = "EXPIRED_LEASE_RECOVERY"
				}
				if err := insertSchedulerEvent(txCtx, session, task, node.NodeCode, eventType, reason,
					map[string]any{"tenantActive": tenantCount, "tenantLimit": tenantMax, "appActive": appCount,
						"appLimit": appMax, "globalActive": nodeCount, "globalLimit": globalMax,
						"fairWeight": fairWeight, "virtualFinish": tenantCandidate.VirtualFinish}); err != nil {
					return err
				}
				if !(task.RetryOfTaskId > 0 && entitlement.ChargeRetryBuild == 0) {
					if err := confirmQuotaInSession(txCtx, session, task.TenantId, "build.count",
						fmt.Sprintf("build:%d", task.Id), "build.started", task.Id,
						billingUsageMetadata(map[string]any{"builderAttempt": newAttempt, "recovered": wasRecovery,
							"retry": task.RetryOfTaskId > 0})); err != nil {
						return err
					}
				}
				if _, _, err := insertOutboxEvent(txCtx, session, task.TenantId, "build.started", "build", task.Id,
					map[string]any{"buildId": task.Id, "appId": task.AppId, "builderId": node.NodeCode,
						"builderAttempt": newAttempt, "recovered": wasRecovery}); err != nil {
					return err
				}
				_, _ = session.ExecCtx(txCtx, `UPDATE t_builder_node SET running_count = ?, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, nodeActive+1, node.Id)
				claimed = task
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &core.BuildTaskResp{Base: okBase(), Data: mapBuildTask(claimed)}, nil
}

func releaseTaskSlot(ctx context.Context, session sqlx.Session, taskID int64, builderAttempt int32, statusValue int64) error {
	_, err := session.ExecCtx(ctx, `UPDATE t_build_slot_lease SET status = ?, released_at = CURRENT_TIMESTAMP(3),
update_time = CURRENT_TIMESTAMP(3) WHERE task_id = ? AND builder_attempt = ? AND status = ?`, statusValue,
		taskID, builderAttempt, buildSlotActive)
	return err
}

func cancelBuildTask(ctx context.Context, svcCtx *svc.ServiceContext, in *core.CancelBuildTaskReq) (*core.BuildTaskResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "cancelled by user"
	}
	if len(reason) > 500 {
		return nil, status.Error(codes.InvalidArgument, "reason is too long")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	var item models.TBuildTask
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &item, buildTaskSelect+` WHERE id = ? AND tenant_id = ? FOR UPDATE`, in.Id, tenant); err != nil {
			return err
		}
		switch item.Status {
		case buildStatusCancelled:
			return nil
		case buildStatusSuccess, buildStatusFailed:
			return status.Error(codes.FailedPrecondition, "finished build task cannot be cancelled")
		}
		previousAttempt := int32(item.BuilderAttempt)
		if previousAttempt == 0 {
			if err := releaseQuotaInSession(txCtx, session, item.TenantId, "build.count", fmt.Sprintf("build:%d", item.Id)); err != nil {
				return err
			}
		}
		if _, err := session.ExecCtx(txCtx, `UPDATE t_build_task SET status = ?, cancel_requested_at = CURRENT_TIMESTAMP(3),
cancelled_at = CURRENT_TIMESTAMP(3), cancel_reason = ?, finish_time = CURRENT_TIMESTAMP(3), lease_until = NULL,
builder_attempt = builder_attempt + 1, update_time = CURRENT_TIMESTAMP(3) WHERE id = ?`, buildStatusCancelled, reason, item.Id); err != nil {
			return err
		}
		if previousAttempt > 0 {
			if err := releaseTaskSlot(txCtx, session, item.Id, previousAttempt, buildSlotCancelled); err != nil {
				return err
			}
		}
		if item.BuilderId.Valid {
			_, _ = session.ExecCtx(txCtx, `UPDATE t_builder_node SET running_count = GREATEST(running_count - 1, 0),
update_time = CURRENT_TIMESTAMP(3) WHERE node_code = ?`, item.BuilderId.String)
		}
		item.Status = buildStatusCancelled
		item.BuilderAttempt++
		item.CancelRequestedAt = nullTime(time.Now())
		item.CancelledAt = item.CancelRequestedAt
		item.CancelReason = nullString(reason)
		item.FinishTime = item.CancelledAt
		if err := insertSchedulerEvent(txCtx, session, &item, stringValue(item.BuilderId),
			core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_CANCELLED, "USER_CANCELLED",
			map[string]any{"reason": reason, "fencedAttempt": previousAttempt}); err != nil {
			return err
		}
		_, _, err := insertOutboxEvent(txCtx, session, item.TenantId, "build.canceled", "build", item.Id,
			map[string]any{"buildId": item.Id, "appId": item.AppId, "reason": reason})
		return err
	})
	if err != nil {
		return nil, notFoundOrInternal(err, "build task")
	}
	return &core.BuildTaskResp{Base: okBase(), Data: mapBuildTask(&item)}, nil
}

func retryBuildTask(ctx context.Context, svcCtx *svc.ServiceContext, in *core.RetryBuildTaskReq) (*core.BuildTaskResp, error) {
	if in == nil || in.Id <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be greater than zero")
	}
	tenant, err := tenantID(ctx)
	if err != nil {
		return nil, err
	}
	var source, created models.TBuildTask
	err = svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
		if err := session.QueryRowCtx(txCtx, &source, buildTaskSelect+` WHERE id = ? AND tenant_id = ? FOR UPDATE`, in.Id, tenant); err != nil {
			return err
		}
		if source.Status != buildStatusFailed && source.Status != buildStatusCancelled {
			return status.Error(codes.FailedPrecondition, "only failed or cancelled build tasks can be retried")
		}
		priority := source.Priority
		if in.Priority > 0 {
			priority = int64(in.Priority)
		}
		_, _, maxPriority, policyErr := schedulerPolicy(txCtx, session, source.TenantId, source.AppId, source.PoolCode)
		if policyErr != nil {
			return policyErr
		}
		if priority > maxPriority {
			return status.Errorf(codes.InvalidArgument, "priority exceeds policy maximum %d", maxPriority)
		}
		result, insertErr := session.ExecCtx(txCtx, `INSERT INTO t_build_task
(tenant_id, app_id, version_id, channel_id, signing_config_id, channel_code, version_code, version_name,
source_apk_object_id, source_apk_url, build_config, branding_profile_id, branding_revision, branding_snapshot,
white_label_product_id, template_revision, template_snapshot, pool_code, cache_key, status, builder_attempt,
priority, queued_at, retry_of_task_id, create_by)
SELECT tenant_id, app_id, version_id, channel_id, signing_config_id, channel_code, version_code, version_name,
source_apk_object_id, source_apk_url, build_config, branding_profile_id, branding_revision, branding_snapshot,
white_label_product_id, template_revision, template_snapshot, pool_code, cache_key, ?, 0, ?, CURRENT_TIMESTAMP(3), id, ?
FROM t_build_task WHERE id = ?`, buildStatusPending, priority, actorID(ctx), source.Id)
		if insertErr != nil {
			return insertErr
		}
		newID, idErr := result.LastInsertId()
		if idErr != nil {
			return idErr
		}
		_, entitlement, _, billingErr := loadTenantBilling(txCtx, session, tenant, true)
		if billingErr != nil {
			return billingErr
		}
		if entitlement.ChargeRetryBuild == 1 {
			if _, err := reserveQuotaInSession(txCtx, session, tenant, "build.count", 1,
				"build", newID, fmt.Sprintf("build:%d", newID), 7*24*time.Hour); err != nil {
				return err
			}
		}
		if err := session.QueryRowCtx(txCtx, &created, buildTaskSelect+` WHERE id = ?`, newID); err != nil {
			return err
		}
		if err := insertSchedulerEvent(txCtx, session, &created, "",
			core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_RETRIED, "MANUAL_RETRY",
			map[string]any{"sourceTaskId": source.Id, "priority": priority}); err != nil {
			return err
		}
		if err := insertSchedulerEvent(txCtx, session, &created, "",
			core.BuildSchedulerEventType_BUILD_SCHEDULER_EVENT_TYPE_QUEUED, "RETRY_QUEUED",
			map[string]any{"retryOfTaskId": source.Id}); err != nil {
			return err
		}
		_, _, err = insertOutboxEvent(txCtx, session, created.TenantId, "build.queued", "build", created.Id,
			map[string]any{"buildId": created.Id, "appId": created.AppId, "retryOfTaskId": source.Id})
		return err
	})
	if err != nil {
		return nil, notFoundOrInternal(err, "build task")
	}
	return &core.BuildTaskResp{Base: okBase(), Data: mapBuildTask(&created)}, nil
}

func schedulerDecisionSummary(node *models.TBuilderNode) string {
	if node == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s running=%d/%d", node.PoolCode, node.NodeCode, node.RunningCount, node.MaxConcurrency)
}
