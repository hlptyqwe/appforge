#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
mysql_container=${APPFORGE_V4_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V4_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V4_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V4_MYSQL_PASSWORD:-appforge_dev_password}
evidence=${APPFORGE_V4_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v4-build-cluster-current-20260815.json}

mysql_scalar() {
  docker exec -e MYSQL_PWD="$mysql_password" "$mysql_container" \
    mysql -u"$mysql_user" -D "$mysql_database" -N -B -e "$1"
}

assert_zero() {
  local label=$1
  local query=$2
  local actual
  actual=$(mysql_scalar "$query")
  if [[ "$actual" != "0" ]]; then
    echo "验收失败: ${label}，异常记录数=${actual}" >&2
    exit 1
  fi
  echo "通过: ${label}"
}

assert_at_least() {
  local label=$1
  local minimum=$2
  local query=$3
  local actual
  actual=$(mysql_scalar "$query")
  if (( actual < minimum )); then
    echo "验收失败: ${label}，实际=${actual}，至少需要=${minimum}" >&2
    exit 1
  fi
  echo "通过: ${label} (${actual})"
}

write_evidence() {
  local current_status=$1 online_nodes=$2 isolated_nodes=$3 minimum_disk_free=$4 minimum_disk_percent=$5
  local successful_builder_nodes policy_count global_limit_events fair_events cache_hit_events cache_failure_events recovery_events cancel_events retry_events
  successful_builder_nodes=$(mysql_scalar "SELECT COUNT(DISTINCT builder_id) FROM t_build_task WHERE status='SUCCESS' AND builder_id IS NOT NULL")
  policy_count=$(mysql_scalar "SELECT COUNT(*) FROM t_build_concurrency_policy WHERE status=1 AND ((tenant_id=0 AND app_id=0) OR (tenant_id>0 AND app_id=0) OR app_id>0)")
  global_limit_events=$(mysql_scalar "SELECT COUNT(*) FROM t_build_scheduler_event WHERE JSON_EXTRACT(decision_json,'$.globalLimit')=1")
  fair_events=$(mysql_scalar "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='FAIR_QUEUE_CLAIM'")
  cache_hit_events=$(mysql_scalar "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='VALIDATED_CACHE_HIT'")
  cache_failure_events=$(mysql_scalar "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='CACHE_DOWNLOAD_VALIDATION_FAILED'")
  recovery_events=$(mysql_scalar "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='EXPIRED_LEASE_RECOVERY'")
  cancel_events=$(mysql_scalar "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='USER_CANCELLED'")
  retry_events=$(mysql_scalar "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='MANUAL_RETRY'")
  mkdir -p "$(dirname "$evidence")"
  umask 077
  EVIDENCE_PATH="$evidence" CURRENT_STATUS="$current_status" ONLINE_NODES="$online_nodes" ISOLATED_NODES="$isolated_nodes" \
  MINIMUM_DISK_FREE="$minimum_disk_free" MINIMUM_DISK_PERCENT="$minimum_disk_percent" \
  SUCCESSFUL_BUILDER_NODES="$successful_builder_nodes" POLICY_COUNT="$policy_count" GLOBAL_LIMIT_EVENTS="$global_limit_events" \
  FAIR_EVENTS="$fair_events" CACHE_HIT_EVENTS="$cache_hit_events" CACHE_FAILURE_EVENTS="$cache_failure_events" \
  RECOVERY_EVENTS="$recovery_events" CANCEL_EVENTS="$cancel_events" RETRY_EVENTS="$retry_events" python3 - <<'PY'
import json, os, pathlib

integer = lambda key: int(os.environ[key])
status = os.environ['CURRENT_STATUS']
passed = status == 'passed'
minimum_disk_free = integer('MINIMUM_DISK_FREE')
minimum_disk_percent = float(os.environ['MINIMUM_DISK_PERCENT'])
failed_disk_gates = []
if minimum_disk_free < 512 * 1024 * 1024:
  failed_disk_gates.append('512 MiB byte threshold')
if minimum_disk_percent < 2:
  failed_disk_gates.append('2 percent threshold')
disk_gate_message = (
  f'Docker VM minimum free space is {minimum_disk_free} bytes/{minimum_disk_percent:.3f} percent and fails '
  + ' and '.join(failed_disk_gates)
  + '; the Core recovery gate requires both thresholds.'
)
payload = {
  'schemaVersion': 1,
  'date': '2026-08-15',
  'mode': 'read-only-current-v4-evidence',
  'databaseMutated': False,
  'currentStatus': status,
  'nodes': {
    'workerContainersRunning': 3,
    'onlineAcceptingNodes': integer('ONLINE_NODES'),
    'isolatedNodes': integer('ISOLATED_NODES'),
    'minimumDiskFreeBytes': minimum_disk_free,
    'minimumDiskFreePercent': minimum_disk_percent,
  },
  'historicalEvidenceCounts': {
    'successfulBuilderNodes': integer('SUCCESSFUL_BUILDER_NODES'),
    'activeConcurrencyPolicyLevels': integer('POLICY_COUNT'),
    'globalLimitOneEvents': integer('GLOBAL_LIMIT_EVENTS'),
    'fairQueueClaims': integer('FAIR_EVENTS'),
    'validatedCacheHits': integer('CACHE_HIT_EVENTS'),
    'cacheValidationFailures': integer('CACHE_FAILURE_EVENTS'),
    'expiredLeaseRecoveries': integer('RECOVERY_EVENTS'),
    'userCancellations': integer('CANCEL_EVENTS'),
    'manualRetries': integer('RETRY_EVENTS'),
  },
  'checks': {
    'threeWorkerContainers': 'passed',
    'threeOnlineAcceptingNodes': 'passed' if passed else 'failed-low-disk-isolation',
    'databaseInvariants': 'passed' if passed else 'not-run-after-node-gate',
  },
  'limitations': [
    'This verifier is read-only and relies on existing synthetic V4 scheduler records.',
    'It does not recreate parallel builds, cancellation, cache corruption or lease failure during this run.',
  ] + ([] if passed else [
    disk_gate_message,
    'Approved Docker cache cleanup and API node recovery are required before current V4 runtime acceptance can pass.',
  ]),
}
path = pathlib.Path(os.environ['EVIDENCE_PATH'])
path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + '\n')
PY
  chmod 0600 "$evidence"
}

for worker in appforge-builder-worker-1 appforge-builder-worker-2 appforge-builder-worker-3; do
  if [[ "$(docker inspect -f '{{.State.Running}}' "$worker" 2>/dev/null)" != "true" ]]; then
    echo "验收失败: Worker 未运行: ${worker}" >&2
    exit 1
  fi
done
echo "通过: 3 个 Docker Builder Worker 均在运行"

fixed_nodes="'builder-dev-01','builder-dev-02','builder-dev-03'"
registered_nodes=$(mysql_scalar "SELECT COUNT(*) FROM t_builder_node WHERE node_code IN ($fixed_nodes)")
[[ $registered_nodes == 3 ]] || { echo "验收失败: 固定开发 Builder 节点登记数为 $registered_nodes，期望 3" >&2; exit 1; }
online_nodes=$(mysql_scalar "SELECT COUNT(*) FROM t_builder_node WHERE node_code IN ($fixed_nodes) AND status=1 AND drain_status=1")
isolated_nodes=$(mysql_scalar "SELECT COUNT(*) FROM t_builder_node WHERE node_code IN ($fixed_nodes) AND status=3")
minimum_disk_free=$(mysql_scalar "SELECT COALESCE(MIN(disk_free),0) FROM t_builder_node WHERE node_code IN ($fixed_nodes)")
minimum_disk_percent=$(mysql_scalar "SELECT COALESCE(ROUND(MIN(disk_free*100.0/NULLIF(disk_capacity,0)),3),0) FROM t_builder_node WHERE node_code IN ($fixed_nodes)")
if (( online_nodes < 3 )); then
  write_evidence failed-low-disk "$online_nodes" "$isolated_nodes" "$minimum_disk_free" "$minimum_disk_percent"
  echo "验收失败: 在线且接收任务的 Builder 节点，实际=${online_nodes}，至少需要=3；最小磁盘余量=${minimum_disk_free}字节/${minimum_disk_percent}%" >&2
  echo "证据: $evidence" >&2
  exit 1
fi
echo "通过: 在线且接收任务的 Builder 节点 (${online_nodes})"
assert_at_least "成功任务实际使用的 Builder 节点" 3 \
  "SELECT COUNT(DISTINCT builder_id) FROM t_build_task WHERE status='SUCCESS' AND builder_id IS NOT NULL;"
assert_at_least "全局/租户/应用三级并发策略" 3 \
  "SELECT COUNT(*) FROM t_build_concurrency_policy WHERE status=1 AND ((tenant_id=0 AND app_id=0) OR (tenant_id>0 AND app_id=0) OR app_id>0);"
assert_at_least "globalLimit=1 的运行时限流证据" 3 \
  "SELECT COUNT(*) FROM t_build_scheduler_event WHERE JSON_EXTRACT(decision_json,'$.globalLimit')=1;"

assert_at_least "公平队列领取事件" 1 \
  "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='FAIR_QUEUE_CLAIM';"
assert_at_least "已验证缓存命中事件" 1 \
  "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='VALIDATED_CACHE_HIT';"
assert_at_least "缓存损坏后自动失效事件" 1 \
  "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='CACHE_DOWNLOAD_VALIDATION_FAILED';"
assert_at_least "租约过期恢复事件" 1 \
  "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='EXPIRED_LEASE_RECOVERY';"
assert_at_least "用户取消事件" 1 \
  "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='USER_CANCELLED';"
assert_at_least "手工重试事件" 1 \
  "SELECT COUNT(*) FROM t_build_scheduler_event WHERE reason_code='MANUAL_RETRY';"

assert_zero "同一任务代次没有重复槽位" \
  "SELECT COUNT(*) FROM (SELECT task_id,builder_attempt FROM t_build_slot_lease GROUP BY task_id,builder_attempt HAVING COUNT(*)>1) duplicate_lease;"
assert_zero "没有非运行任务占用生效槽位" \
  "SELECT COUNT(*) FROM t_build_slot_lease l JOIN t_build_task t ON t.id=l.task_id WHERE l.status=1 AND t.status NOT IN ('BUILDING','SIGNING','UPLOADING');"
assert_zero "没有运行任务缺失生效槽位" \
  "SELECT COUNT(*) FROM t_build_task t LEFT JOIN t_build_slot_lease l ON l.task_id=t.id AND l.builder_attempt=t.builder_attempt AND l.status=1 WHERE t.status IN ('BUILDING','SIGNING','UPLOADING') AND l.id IS NULL;"
assert_zero "成功任务都有 APK 产物" \
  "SELECT COUNT(*) FROM t_build_task WHERE status='SUCCESS' AND apk_object_id=0;"
assert_zero "任务与 APK 元数据一致" \
  "SELECT COUNT(*) FROM t_build_task t JOIN t_storage_object o ON o.id=t.apk_object_id WHERE t.status='SUCCESS' AND (t.apk_sha256<>o.sha256 OR t.apk_size<>o.size_bytes);"

assert_zero "缓存对象没有跨租户引用" \
  "SELECT COUNT(*) FROM t_build_cache_entry c JOIN t_storage_object o ON o.id=c.artifact_object_id WHERE c.tenant_id<>o.tenant_id;"
assert_zero "源 APK 没有跨租户引用" \
  "SELECT COUNT(*) FROM t_build_task t JOIN t_storage_object o ON o.id=t.source_apk_object_id WHERE t.tenant_id<>o.tenant_id;"
assert_zero "构建 APK 没有跨租户引用" \
  "SELECT COUNT(*) FROM t_build_task t JOIN t_storage_object o ON o.id=t.apk_object_id WHERE t.apk_object_id>0 AND t.tenant_id<>o.tenant_id;"
assert_zero "构建日志没有跨租户引用" \
  "SELECT COUNT(*) FROM t_build_task t JOIN t_storage_object o ON o.id=t.log_object_id WHERE t.log_object_id>0 AND t.tenant_id<>o.tenant_id;"
assert_zero "签名配置没有跨租户引用" \
  "SELECT COUNT(*) FROM t_build_task t JOIN t_app_signing_config s ON s.id=t.signing_config_id WHERE t.tenant_id<>s.tenant_id;"
assert_zero "APK 对象 Key 遵守租户前缀" \
  "SELECT COUNT(*) FROM t_build_task t JOIN t_storage_object o ON o.id=t.apk_object_id WHERE t.apk_object_id>0 AND o.object_key NOT LIKE CONCAT('tenants/',t.tenant_id,'/build-apk/%');"
assert_zero "日志对象 Key 遵守租户前缀" \
  "SELECT COUNT(*) FROM t_build_task t JOIN t_storage_object o ON o.id=t.log_object_id WHERE t.log_object_id>0 AND o.object_key NOT LIKE CONCAT('tenants/',t.tenant_id,'/build-log/%');"

write_evidence passed "$online_nodes" "$isolated_nodes" "$minimum_disk_free" "$minimum_disk_percent"
echo "V4 构建集群只读验收检查通过"
echo "证据: $evidence"
