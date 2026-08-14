#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
mysql_container=${APPFORGE_V6_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V6_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V6_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V6_MYSQL_PASSWORD:-appforge_dev_password}
admin_ui_url=${APPFORGE_V6_ADMIN_UI_URL:-http://127.0.0.1:5173}
agent_ui_url=${APPFORGE_V6_AGENT_UI_URL:-http://127.0.0.1:5174}
api_url=${APPFORGE_V6_API_URL:-http://127.0.0.1:8888}
go_cache=${APPFORGE_V6_GO_CACHE:-/private/tmp/appforge-go-build-cache}

mysql_scalar() {
  docker exec -e MYSQL_PWD="$mysql_password" "$mysql_container" \
    mysql -u"$mysql_user" -D "$mysql_database" -N -B -e "$1"
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

assert_zero() {
  assert_equal "$1" 0 "$2"
}

assert_healthy() {
  local container=$1
  if [[ "$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container" 2>/dev/null)" != "healthy" ]]; then
    echo "验收失败: 容器不健康: ${container}" >&2
    exit 1
  fi
  echo "通过: 容器健康 ${container}"
}

assert_running() {
  local container=$1
  if [[ "$(docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null)" != "true" ]]; then
    echo "验收失败: 容器未运行: ${container}" >&2
    exit 1
  fi
  echo "通过: 容器正在运行 ${container}"
}

for container in appforge-admin-api appforge-admin-ui appforge-agent-ui appforge-core-rpc; do
  assert_healthy "$container"
done
assert_running appforge-billing-worker

curl -fsS -o /dev/null "$admin_ui_url/"
curl -fsS -o /dev/null "$agent_ui_url/"
curl -fsS -o /dev/null "$api_url/admin/system/core"
echo "通过: 管理端、代理端和 API 健康入口可访问"

assert_equal "V6 数据库迁移完整" 2 \
  "SELECT COUNT(*) FROM sys_schema_migration WHERE version IN ('20260814_106_v6_commercialization','20260814_107_v6_billing_menu');"
assert_equal "V6 核心业务表完整" 10 \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('t_billing_plan','t_tenant_subscription','t_tenant_entitlement','t_usage_ledger','t_quota_reservation','t_usage_threshold_notification','t_invoice','t_invoice_item','t_payment_transaction','t_billing_webhook_event');"
assert_equal "Free、Pro、Business 套餐种子完整" 3 \
  "SELECT COUNT(DISTINCT plan_code) FROM t_billing_plan WHERE status=1 AND plan_code IN ('free','pro','business');"
assert_zero "现有租户全部具有订阅" \
  "SELECT COUNT(*) FROM sys_tenant t LEFT JOIN t_tenant_subscription s ON s.tenant_id=t.id WHERE s.id IS NULL;"
assert_zero "现有租户全部具有权益快照" \
  "SELECT COUNT(*) FROM sys_tenant t LEFT JOIN t_tenant_entitlement e ON e.tenant_id=t.id WHERE e.id IS NULL;"
assert_zero "同一租户不存在多份当前订阅" \
  "SELECT COUNT(*) FROM (SELECT tenant_id FROM t_tenant_subscription GROUP BY tenant_id HAVING COUNT(*)>1) duplicates;"
assert_zero "用量账本幂等键没有重复" \
  "SELECT COUNT(*) FROM (SELECT tenant_id,metric,idempotency_key FROM t_usage_ledger GROUP BY tenant_id,metric,idempotency_key HAVING COUNT(*)>1) duplicates;"
assert_zero "额度预占幂等键没有重复" \
  "SELECT COUNT(*) FROM (SELECT tenant_id,metric,idempotency_key FROM t_quota_reservation GROUP BY tenant_id,metric,idempotency_key HAVING COUNT(*)>1) duplicates;"
assert_zero "支付事件没有明文 payload 列" \
  "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='t_billing_webhook_event' AND column_name IN ('payload','payload_json','raw_payload');"
assert_zero "账单金额字段没有浮点类型" \
  "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name IN ('t_billing_plan','t_invoice','t_invoice_item','t_payment_transaction') AND column_name LIKE '%amount%' AND data_type IN ('float','double','decimal','numeric','real');"

(
  cd "$repo_root/appforge-api"
  GOCACHE="$go_cache" go test ./internal/handler -run 'Test(VerifyStripeSignature|NormalizeStripeSubscriptionEvent)' -count=1
)
(
  cd "$repo_root/services/core"
  APPFORGE_BILLING_TEST_DSN="${mysql_user}:${mysql_password}@tcp(127.0.0.1:3306)/${mysql_database}?charset=utf8mb4&parseTime=true&loc=Local" \
    GOCACHE="$go_cache" go test ./internal/logic -run TestBillingRuntimeAcceptance -count=1
)
echo "通过: Stripe 验签、并发额度、重复/乱序事件、退款与争议运行时验收"

echo "V6 商业化验收检查通过"
