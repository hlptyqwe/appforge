#!/usr/bin/env bash

set -euo pipefail

mysql_container=${APPFORGE_V4_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V4_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V4_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V4_MYSQL_PASSWORD:-appforge_dev_password}
api_url=${APPFORGE_V4_API_URL:-http://127.0.0.1:8888}
admin_username=${APPFORGE_V4_ADMIN_USERNAME:-appforge}
admin_password=${APPFORGE_V4_ADMIN_PASSWORD:-AppForge@123}

mysql_query() {
  docker exec -e MYSQL_PWD="$mysql_password" "$mysql_container" \
    mysql -u"$mysql_user" -D "$mysql_database" -N -B -e "$1"
}

nodes=$(mysql_query "SELECT id,node_code,status FROM t_builder_node WHERE node_code IN ('builder-dev-01','builder-dev-02','builder-dev-03') ORDER BY node_code;")
[[ $(printf '%s\n' "$nodes" | awk 'NF {count++} END {print count+0}') == 3 ]] || {
  echo "验收失败: 未找到三个固定开发 Builder 节点" >&2
  exit 1
}

login_response=$(curl -fsS -H 'Content-Type: application/json' \
  -d "{\"username\":\"${admin_username}\",\"password\":\"${admin_password}\"}" \
  "$api_url/admin/system/auth/login")
admin_token=$(printf '%s' "$login_response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')

while read -r node_id node_code node_status; do
  case "$node_status" in
    1)
      echo "保持: $node_code 已在线"
      ;;
    3)
      response=$(curl -fsS -X POST -H "Authorization: Bearer $admin_token" -H 'Content-Type: application/json' \
        -d '{"reason":"V1-V7 regression precondition"}' \
        "$api_url/admin/core/build-cluster/nodes/$node_id/recover")
      printf '%s' "$response" | python3 -c '
import json,sys
payload=json.load(sys.stdin)
assert payload["code"]==200 and payload["data"]["status"]==1
'
      echo "通过真实 API 恢复: $node_code"
      ;;
    *)
      echo "验收失败: $node_code 状态为 $node_status，仅允许在线或人工隔离状态" >&2
      exit 1
      ;;
  esac
done <<<"$nodes"

online=$(mysql_query "SELECT COUNT(*) FROM t_builder_node WHERE node_code IN ('builder-dev-01','builder-dev-02','builder-dev-03') AND status=1 AND drain_status=1;")
[[ $online == 3 ]] || { echo "验收失败: 恢复后在线接单节点数为 $online" >&2; exit 1; }

echo "通过: 三个固定开发 Builder 节点经登录、RBAC、HTTP、Core RPC 和恢复前置检查后在线接单"
