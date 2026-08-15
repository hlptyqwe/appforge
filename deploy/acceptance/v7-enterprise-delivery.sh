#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
mysql_container=${APPFORGE_V7_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V7_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V7_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V7_MYSQL_PASSWORD:-appforge_dev_password}
api_url=${APPFORGE_V7_API_URL:-http://127.0.0.1:8888}
admin_ui_url=${APPFORGE_V7_ADMIN_UI_URL:-http://127.0.0.1:5173}
agent_ui_url=${APPFORGE_V7_AGENT_UI_URL:-http://127.0.0.1:5174}
admin_username=${APPFORGE_V7_ADMIN_USERNAME:-appforge}
admin_password=${APPFORGE_V7_ADMIN_PASSWORD:-AppForge@123}
go_cache=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache}
go_mod_cache=${APPFORGE_V7_GO_MOD_CACHE:-/private/tmp/appforge-go-mod-cache}

mysql_scalar() {
  docker exec -e MYSQL_PWD="$mysql_password" "$mysql_container" \
    mysql -u"$mysql_user" -D "$mysql_database" -N -B -e "$1"
}

assert_equal() {
  local label=$1 expected=$2 query=$3 actual
  actual=$(mysql_scalar "$query")
  [[ "$actual" == "$expected" ]] || { echo "验收失败: ${label}，实际=${actual}，期望=$expected" >&2; exit 1; }
  echo "通过: $label ($actual)"
}

assert_zero() { assert_equal "$1" 0 "$2"; }

for container in appforge-admin-api appforge-admin-ui appforge-agent-ui appforge-core-rpc; do
  [[ $(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container" 2>/dev/null) == healthy ]] || {
    echo "验收失败: 容器不健康: $container" >&2; exit 1;
  }
done
[[ $(docker inspect -f '{{.State.Running}}' appforge-enterprise-worker 2>/dev/null) == true ]] || {
  echo "验收失败: enterprise worker未运行" >&2; exit 1;
}

curl -fsS "$api_url/healthz" >/dev/null
curl -fsS "$api_url/readyz" >/dev/null
curl -fsS "$admin_ui_url/" >/dev/null
curl -fsS "$agent_ui_url/" >/dev/null
echo "通过: API健康探针与两套现有前端可访问"

unauthorized_code=$(curl -sS -o /dev/null -w '%{http_code}' "$api_url/admin/core/enterprise/deployment")
[[ "$unauthorized_code" == "401" ]] || { echo "验收失败: 未认证部署状态请求未被拒绝" >&2; exit 1; }
login_response=$(curl -fsS -H 'Content-Type: application/json' \
  -d "{\"username\":\"${admin_username}\",\"password\":\"${admin_password}\"}" \
  "$api_url/admin/system/auth/login")
admin_token=$(printf '%s' "$login_response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')
profile_response=$(curl -fsS -H "Authorization: Bearer $admin_token" "$api_url/admin/system/auth/profile")
printf '%s' "$profile_response" | python3 -c '
import json,sys
payload=json.load(sys.stdin)["data"]
assert payload["user"]["userType"]==1 and payload["user"]["tenantId"]==0
def walk(nodes):
  for node in nodes:
    yield node
    yield from walk(node.get("children", []))
assert sum(1 for node in walk(payload.get("menus", [])) if node.get("path")=="/platform/deployment")==1
assert "enterprise:deployment:view" in payload.get("perms", [])
'
deployment_response=$(curl -fsS -H "Authorization: Bearer $admin_token" "$api_url/admin/core/enterprise/deployment")
printf '%s' "$deployment_response" | python3 -c '
import json,sys
payload=json.load(sys.stdin)
data=payload["data"]
assert payload["code"]==200
assert data["actualSchemaVersion"]=="20260815_111_v7_deployment_console"
assert data["schemaCompatible"] is True and data["upgradeReady"] is True
assert len(data["components"])==5 and all(item["status"]=="healthy" for item in data["components"])
assert "20260815_111_v7_deployment_console" in [item["version"] for item in data["recentMigrations"]]
'
unset admin_token login_response profile_response deployment_response
echo "通过: 企业部署页面真实认证、RBAC、组件健康、许可证、Schema和升级前置状态"

local_agent_view="$repo_root/appforge-ui/src/views/platform/local-agents.vue"
rg -q -- '--token-stdin' "$local_agent_view"
if rg -q -- '--token[[:space:]]' "$local_agent_view"; then
  echo "验收失败: Local Agent 注册命令不得把一次性注册码放入进程参数" >&2
  exit 1
fi
rg -q '24小时内到期' "$local_agent_view"
echo "通过: Local Agent 注册码仅从标准输入传递，管理页包含证书临期告警"

assert_equal "V7数据库迁移及管理权限完整" 4 \
  "SELECT COUNT(*) FROM sys_schema_migration WHERE version IN ('20260814_108_v7_enterprise','20260814_109_v7_enterprise_menu','20260814_110_v4_builder_node_recovery','20260815_111_v7_deployment_console');"
assert_equal "V7企业部署页面及只读接口权限完整" 2 \
  "SELECT COUNT(*) FROM sys_menu WHERE id IN (4700,4710) AND enabled=1;"
assert_zero "V7企业部署信息未授予业务租户用户" \
  "SELECT COUNT(*) FROM sys_role_menu rm JOIN sys_user_role ur ON ur.role_id=rm.role_id JOIN sys_user u ON u.id=ur.user_id WHERE rm.menu_id IN (4700,4710) AND (u.tenant_id<>0 OR u.user_type<>1);"
assert_equal "V7企业部署信息已授予平台Owner" 2 \
  "SELECT COUNT(DISTINCT rm.menu_id) FROM sys_role_menu rm JOIN sys_user_role ur ON ur.role_id=rm.role_id JOIN sys_user u ON u.id=ur.user_id WHERE rm.menu_id IN (4700,4710) AND u.tenant_id=0 AND u.app_scope=1 AND u.user_type=1;"
assert_equal "V7 Local Agent业务表完整" 5 \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('t_local_agent','t_local_agent_registration','t_local_agent_certificate','t_local_agent_capability','t_hybrid_artifact_reference');"
assert_zero "Agent证书表不保存私钥" \
  "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='t_local_agent_certificate' AND column_name LIKE '%private%';"
assert_zero "一次性注册码只保存SHA-256摘要" \
  "SELECT COUNT(*) FROM t_local_agent_registration WHERE CHAR_LENGTH(token_hash)<>64 OR token_hash NOT REGEXP '^[0-9a-f]{64}$';"
assert_zero "Local Agent授权应用不存在跨租户引用" \
  "SELECT COUNT(*) FROM t_local_agent a JOIN JSON_TABLE(a.allowed_app_ids, '\$[*]' COLUMNS(app_id BIGINT PATH '\$')) ids JOIN t_app_application app ON app.id=ids.app_id WHERE app.tenant_id<>a.tenant_id;"
assert_zero "Hybrid Artifact不存在跨租户任务引用" \
  "SELECT COUNT(*) FROM t_hybrid_artifact_reference h JOIN t_build_task t ON t.id=h.task_id WHERE h.tenant_id<>t.tenant_id OR h.builder_attempt<>t.builder_attempt;"
assert_zero "Hybrid Artifact引用不包含常见URL凭证" \
  "SELECT COUNT(*) FROM t_hybrid_artifact_reference WHERE object_reference LIKE '%@%' OR object_reference REGEXP '[[:cntrl:]]';"

(
  cd "$repo_root/common"
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./secretprovider ./offlinelicense -count=1
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test -race ./siem -count=1
)
(
  cd "$repo_root/local-agent"
  GOCACHE="$go_cache" go test -race ./... -count=1
)
(
  cd "$repo_root/services/core"
  APPFORGE_ENTERPRISE_TEST_DSN="${mysql_user}:${mysql_password}@tcp(127.0.0.1:3306)/${mysql_database}?charset=utf8mb4&parseTime=true&loc=Asia%2FHong_Kong" \
    GOCACHE="$go_cache" go test ./internal/agentpki ./internal/logic -run 'Test(LocalAgentRuntimeAcceptance|SignCSR)' -count=1
)

license_tmp=$(mktemp -d)
trap 'rm -rf "$license_tmp"' EXIT
(
  cd "$repo_root/appforge-api"
  GOCACHE="$go_cache" go run ./cmd/licensectl generate-key --private "$license_tmp/vendor-private.pem" --public "$license_tmp/vendor-public.pem"
  GOCACHE="$go_cache" go run ./cmd/licensectl issue --private "$license_tmp/vendor-private.pem" --output "$license_tmp/license.json" \
    --license-id v7-acceptance --customer acceptance --deployment-id acceptance-private --modes private --valid-for 24h
  GOCACHE="$go_cache" go run ./cmd/licensectl verify --license "$license_tmp/license.json" --public "$license_tmp/vendor-public.pem" \
    --state "$license_tmp/state/state.json" --deployment-id acceptance-private --mode private >/dev/null
)
echo "通过: 断网许可证密钥生成、签发、部署绑定和持久状态校验"

APPFORGE_DIAGNOSTIC_API_URL="$api_url" "$repo_root/deploy/acceptance/v7-diagnostics.sh"
"$repo_root/deploy/acceptance/v7-production-preflight.sh"
"$repo_root/deploy/acceptance/v7-offline-layout.sh"
"$repo_root/deploy/acceptance/v7-release-workflow.sh"
"$repo_root/deploy/acceptance/v7-network-policy.sh"
"$repo_root/deploy/acceptance/v7-backup-restore.sh"

bash -n "$repo_root/deploy/production/"*.sh "$repo_root/deploy/local-agent/"*.sh
bash -n "$repo_root/deploy/acceptance/v7-local-agent-recovery.sh" \
  "$repo_root/deploy/acceptance/v7-kubernetes-upgrade.sh" \
  "$repo_root/deploy/acceptance/v7-network-policy-runtime.sh"
docker compose --env-file "$repo_root/deploy/production/.env.example" -f "$repo_root/deploy/production/docker-compose.yml" config --quiet
docker compose --env-file "$repo_root/deploy/local-agent/.env.example" -f "$repo_root/deploy/local-agent/docker-compose.yml" \
  --profile registration --profile maintenance config --quiet
python3 -m json.tool "$repo_root/deploy/helm/appforge/values.schema.json" >/dev/null
docker run --rm -v "$repo_root/deploy/helm:/charts" alpine/helm:3.17.3 lint /charts/appforge --strict \
  --set global.imageRegistry=registry.example.com/appforge --set global.publicOrigin=https://appforge.example.com \
  --set image.tag=1.0.0 --set ingress.host=appforge.example.com --set ingress.adminHost=admin.appforge.example.com \
  --set offlineLicense.deploymentId=acceptance-private --set offlineLicense.existingStateClaim=appforge-license-state \
  --set observability.siemWebhook=https://siem.example.com/appforge/audit >/dev/null
echo "通过: Compose、离线脚本、Helm values schema和严格lint"

echo "V7 企业交付基础验收通过；三种Artifact端到端构建、真实AWS、完整业务灾备和正式发布证据仍需独立运行。"
