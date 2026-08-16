#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
mysql_container=${APPFORGE_V5_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V5_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V5_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V5_MYSQL_PASSWORD:-appforge_dev_password}
agent_ui_url=${APPFORGE_V5_AGENT_UI_URL:-http://127.0.0.1:5174}
admin_ui_url=${APPFORGE_V5_ADMIN_UI_URL:-http://127.0.0.1:5173}
evidence=${APPFORGE_V5_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v5-open-platform-readonly-20260815.json}

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

assert_equal() {
  local label=$1
  local expected=$2
  local query=$3
  local actual
  actual=$(mysql_scalar "$query")
  if [[ "$actual" != "$expected" ]]; then
    echo "验收失败: ${label}，实际=${actual}，期望=${expected}" >&2
    exit 1
  fi
  echo "通过: ${label} (${actual})"
}

assert_running() {
  local container=$1
  if [[ "$(docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null)" != "true" ]]; then
    echo "验收失败: 容器未运行: ${container}" >&2
    exit 1
  fi
  echo "通过: 容器正在运行 ${container}"
}

assert_healthy() {
  local container=$1
  if [[ "$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container" 2>/dev/null)" != "healthy" ]]; then
    echo "验收失败: 容器不健康: ${container}" >&2
    exit 1
  fi
  echo "通过: 容器健康 ${container}"
}

for container in appforge-webhook-worker appforge-source-trigger-worker appforge-core-rpc; do
  assert_running "$container"
done
for container in appforge-admin-api appforge-admin-ui appforge-agent-ui; do
  assert_healthy "$container"
done

curl -fsS -o /dev/null "$admin_ui_url/"
curl -fsS -o /dev/null "$agent_ui_url/"
curl -fsS -o /dev/null "$agent_ui_url/agent/core"
echo "通过: 管理端、代理端及代理 API 转发可访问"

assert_equal "V5 数据库迁移完整" 6 \
  "SELECT COUNT(*) FROM sys_schema_migration WHERE version IN ('20260814_100_v5_open_platform','20260814_101_v5_developer_menu','20260814_102_v5_webhook_permissions','20260814_103_v5_source_integration_permissions','20260814_104_v5_source_build_triggers','20260814_105_v5_source_trigger_permissions');"
assert_equal "V5 核心业务表完整" 11 \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('t_open_api_credential','t_open_api_idempotency','t_open_api_audit','t_webhook_endpoint','t_outbox_event','t_webhook_delivery','t_source_integration','t_source_repository','t_source_artifact','t_source_build_trigger','t_source_webhook_event');"

assert_at_least "API Key 运行时验收凭证" 1 \
  "SELECT COUNT(*) FROM t_open_api_credential;"
assert_zero "API Key 只保存 64 位 SHA-256 摘要" \
  "SELECT COUNT(*) FROM t_open_api_credential WHERE CHAR_LENGTH(secret_hash)<>64 OR secret_hash NOT REGEXP '^[0-9a-f]{64}$';"
assert_zero "API Key 数据表没有明文密钥列" \
  "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='t_open_api_credential' AND column_name IN ('api_key','secret','secret_plaintext','credential_secret');"

assert_at_least "Open API 幂等完成记录" 1 \
  "SELECT COUNT(*) FROM t_open_api_idempotency WHERE status=2 AND resource_id>0;"
assert_zero "Open API 幂等键没有重复记录" \
  "SELECT COUNT(*) FROM (SELECT tenant_id,credential_id,request_method,request_path,idempotency_key FROM t_open_api_idempotency GROUP BY tenant_id,credential_id,request_method,request_path,idempotency_key HAVING COUNT(*)>1) duplicate_idempotency;"
assert_at_least "Open API 调用审计" 1 \
  "SELECT COUNT(*) FROM t_open_api_audit WHERE request_id<>'' AND request_path LIKE '/open/v1/%';"
assert_at_least "Open API 鉴权拒绝审计" 1 \
  "SELECT COUNT(*) FROM t_open_api_audit WHERE response_status=403;"
assert_at_least "Open API 幂等冲突审计" 1 \
  "SELECT COUNT(*) FROM t_open_api_audit WHERE response_status=409;"

assert_equal "构建生命周期 Outbox 核心事件齐全" 4 \
  "SELECT COUNT(DISTINCT event_type) FROM t_outbox_event WHERE event_type IN ('build.queued','build.started','build.succeeded','build.failed');"
assert_zero "Webhook 投递没有事件端点重复记录" \
  "SELECT COUNT(*) FROM (SELECT outbox_event_id,endpoint_id FROM t_webhook_delivery GROUP BY outbox_event_id,endpoint_id HAVING COUNT(*)>1) duplicate_delivery;"
assert_at_least "Webhook SSRF 阻断并进入死信" 1 \
  "SELECT COUNT(*) FROM t_webhook_delivery WHERE status=5 AND (LOWER(error_message) LIKE '%private%' OR LOWER(error_message) LIKE '%reserved%' OR LOWER(error_message) LIKE '%loopback%');"

assert_equal "GitHub 与 GitLab 集成运行时证据" 2 \
  "SELECT COUNT(DISTINCT platform) FROM t_source_integration WHERE platform IN (1,2);"
assert_equal "GitHub 与 GitLab 入站事件运行时证据" 2 \
  "SELECT COUNT(DISTINCT i.platform) FROM t_source_webhook_event e JOIN t_source_build_trigger t ON t.id=e.trigger_id JOIN t_source_repository r ON r.id=t.repository_id JOIN t_source_integration i ON i.id=r.integration_id WHERE i.platform IN (1,2);"
assert_zero "源码 Webhook 供应商投递号没有重复" \
  "SELECT COUNT(*) FROM (SELECT trigger_id,provider_event_id FROM t_source_webhook_event GROUP BY trigger_id,provider_event_id HAVING COUNT(*)>1) duplicate_source_event;"
assert_zero "源码事件每个渠道最多创建一个构建" \
  "SELECT COUNT(*) FROM (SELECT source_webhook_event_id,channel_id FROM t_build_task WHERE source_webhook_event_id IS NOT NULL GROUP BY source_webhook_event_id,channel_id HAVING COUNT(*)>1) duplicate_source_build;"
assert_zero "V5 验收用源码集成和触发器均已停用" \
  "SELECT (SELECT COUNT(*) FROM t_source_integration WHERE integration_name LIKE 'V5 %acceptance%' AND status=1) + (SELECT COUNT(*) FROM t_source_build_trigger WHERE trigger_name LIKE 'V5 %acceptance%' AND status=1);"

assert_at_least "开发者中心写操作审计" 1 \
  "SELECT COUNT(*) FROM sys_op_log WHERE path LIKE '%/developer/%' AND method IN ('POST','PUT','DELETE');"
assert_zero "操作日志没有 API Key 明文特征" \
  "SELECT COUNT(*) FROM sys_op_log WHERE req REGEXP 'af_[A-Za-z0-9_-]{20,}' OR resp REGEXP 'af_[A-Za-z0-9_-]{20,}';"
assert_zero "操作日志没有源码 Webhook 路径令牌" \
  "SELECT COUNT(*) FROM sys_op_log WHERE req LIKE '%/source-webhooks/%' OR resp LIKE '%/source-webhooks/%';"

credential_count=$(mysql_scalar "SELECT COUNT(*) FROM t_open_api_credential")
idempotency_count=$(mysql_scalar "SELECT COUNT(*) FROM t_open_api_idempotency WHERE status=2 AND resource_id>0")
audit_count=$(mysql_scalar "SELECT COUNT(*) FROM t_open_api_audit WHERE request_id<>'' AND request_path LIKE '/open/v1/%'")
forbidden_audit_count=$(mysql_scalar "SELECT COUNT(*) FROM t_open_api_audit WHERE response_status=403")
conflict_audit_count=$(mysql_scalar "SELECT COUNT(*) FROM t_open_api_audit WHERE response_status=409")
outbox_event_type_count=$(mysql_scalar "SELECT COUNT(DISTINCT event_type) FROM t_outbox_event WHERE event_type IN ('build.queued','build.started','build.succeeded','build.failed')")
ssrf_dead_letter_count=$(mysql_scalar "SELECT COUNT(*) FROM t_webhook_delivery WHERE status=5 AND (LOWER(error_message) LIKE '%private%' OR LOWER(error_message) LIKE '%reserved%' OR LOWER(error_message) LIKE '%loopback%')")
source_platform_count=$(mysql_scalar "SELECT COUNT(DISTINCT platform) FROM t_source_integration WHERE platform IN (1,2)")
source_event_platform_count=$(mysql_scalar "SELECT COUNT(DISTINCT i.platform) FROM t_source_webhook_event e JOIN t_source_build_trigger t ON t.id=e.trigger_id JOIN t_source_repository r ON r.id=t.repository_id JOIN t_source_integration i ON i.id=r.integration_id WHERE i.platform IN (1,2)")
developer_audit_count=$(mysql_scalar "SELECT COUNT(*) FROM sys_op_log WHERE path LIKE '%/developer/%' AND method IN ('POST','PUT','DELETE')")

mkdir -p "$(dirname "$evidence")"
umask 077
EVIDENCE_PATH="$evidence" CREDENTIAL_COUNT="$credential_count" IDEMPOTENCY_COUNT="$idempotency_count" \
AUDIT_COUNT="$audit_count" FORBIDDEN_AUDIT_COUNT="$forbidden_audit_count" CONFLICT_AUDIT_COUNT="$conflict_audit_count" \
OUTBOX_EVENT_TYPE_COUNT="$outbox_event_type_count" SSRF_DEAD_LETTER_COUNT="$ssrf_dead_letter_count" \
SOURCE_PLATFORM_COUNT="$source_platform_count" SOURCE_EVENT_PLATFORM_COUNT="$source_event_platform_count" \
DEVELOPER_AUDIT_COUNT="$developer_audit_count" python3 - <<'PY'
import json, os, pathlib

integer = lambda key: int(os.environ[key])
payload = {
  'schemaVersion': 1,
  'date': '2026-08-15',
  'mode': 'read-only-existing-synthetic-v5-evidence',
  'databaseMutated': False,
  'counts': {
    'apiCredentials': integer('CREDENTIAL_COUNT'),
    'completedIdempotencyRecords': integer('IDEMPOTENCY_COUNT'),
    'openApiAudits': integer('AUDIT_COUNT'),
    'forbiddenAudits': integer('FORBIDDEN_AUDIT_COUNT'),
    'conflictAudits': integer('CONFLICT_AUDIT_COUNT'),
    'buildOutboxEventTypes': integer('OUTBOX_EVENT_TYPE_COUNT'),
    'ssrfDeadLetters': integer('SSRF_DEAD_LETTER_COUNT'),
    'sourcePlatforms': integer('SOURCE_PLATFORM_COUNT'),
    'sourceEventPlatforms': integer('SOURCE_EVENT_PLATFORM_COUNT'),
    'developerWriteAudits': integer('DEVELOPER_AUDIT_COUNT'),
  },
  'checks': {
    'requiredContainersAndUiRoutes': 'passed',
    'schemaMigrationsAndTables': 'passed',
    'apiSecretHashOnly': 'passed',
    'idempotencyUniqueness': 'passed',
    'authorizationAndConflictAudits': 'passed',
    'outboxEventCoverage': 'passed',
    'webhookDeliveryUniqueness': 'passed',
    'webhookSsrfDeadLetter': 'passed',
    'githubGitlabEvidence': 'passed',
    'sourceEventAndBuildIdempotency': 'passed',
    'acceptanceIntegrationsDisabled': 'passed',
    'operationLogSecretScan': 'passed',
  },
  'limitations': [
    'This verifier is read-only and relies on existing synthetic V5 runtime records.',
    'It does not create, rotate or revoke a live API key during this run.',
    'It does not replay the CLI upload-build-download flow during this run.',
    'GitHub and GitLab evidence uses historical synthetic signed events; no real provider credentials are accessed.',
  ],
}
path = pathlib.Path(os.environ['EVIDENCE_PATH'])
path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + '\n')
PY
chmod 0600 "$evidence"

echo "V5 开放平台只读验收检查通过"
echo "证据: $evidence"
