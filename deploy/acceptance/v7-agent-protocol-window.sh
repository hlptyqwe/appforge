#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
mysql_container=${APPFORGE_V7_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V7_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V7_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V7_MYSQL_PASSWORD:-appforge_dev_password}
report_file=${APPFORGE_AGENT_PROTOCOL_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-agent-protocol-window-20260817.json}
go_cache=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-agent-protocol-cache}

[[ $report_file == /* ]] || { echo "验收失败: 证据路径必须是绝对路径" >&2; exit 1; }
for command_name in docker go jq git; do
  command -v "$command_name" >/dev/null || {
    echo "验收失败: 缺少命令 $command_name" >&2
    exit 1
  }
done
[[ $(docker inspect -f '{{.State.Running}}' "$mysql_container" 2>/dev/null) == true ]] || {
  echo "验收失败: MySQL容器未运行: $mysql_container" >&2
  exit 1
}

mkdir -p "$(dirname "$report_file")" "$go_cache"
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
umask 077

(
  umask 022
  cd "$repo_root/common"
  GOCACHE="$go_cache" go test ./agentprotocol -count=1
)
(
  umask 022
  cd "$repo_root/appforge-api"
  GOCACHE="$go_cache" go test ./internal/config -count=1
)
(
  umask 022
  cd "$repo_root/local-agent"
  GOCACHE="$go_cache" go test ./... -count=1
)
(
  umask 022
  cd "$repo_root/services/core"
  APPFORGE_ENTERPRISE_TEST_DSN="${mysql_user}:${mysql_password}@tcp(127.0.0.1:3306)/${mysql_database}?charset=utf8mb4&parseTime=true&loc=Asia%2FHong_Kong" \
    GOCACHE="$go_cache" go test ./internal/logic -run '^TestLocalAgentRuntimeAcceptance$' -count=1
)

goos=$(go env GOOS)
goarch=$(go env GOARCH)
git_commit=$(git -C "$repo_root" rev-parse HEAD)
jq -n \
  --arg acceptedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg goos "$goos" \
  --arg goarch "$goarch" \
  --arg gitCommit "$git_commit" \
  '{
    schemaVersion: 1,
    evidenceType: "v7-local-agent-protocol-window",
    acceptedAt: $acceptedAt,
    result: "passed",
    source: {gitCommit: $gitCommit},
    runtime: {goos: $goos, goarch: $goarch, database: "temporary-test-records-on-local-docker-mysql"},
    compatibilityWindow: {
      minimumProtocol: 2,
      currentProtocol: 3,
      taskBundleProtocol: 3,
      protocol1: {heartbeat: "upgrade-required", certificateRotation: "allowed", newTaskClaim: "rejected"},
      protocol2: {heartbeat: "online", newTaskClaim: "rejected-task-bundle-unsupported"},
      protocol3: {heartbeat: "online", newTaskClaim: "allowed"},
      protocol4: {heartbeat: "upgrade-required", newTaskClaim: "rejected-future-protocol"}
    },
    verified: [
      "single-shared-protocol-contract",
      "deployment-window-configuration-drift-rejected",
      "unsupported-agents-retain-safe-heartbeat-and-certificate-rotation",
      "unsupported-agents-cannot-claim-new-tasks",
      "supported-legacy-agent-cannot-claim-task-bundle-v3",
      "current-agent-can-claim-and-complete-runtime-lifecycle",
      "air-gapped-export-and-import-use-current-protocol-contract",
      "temporary-agent-certificate-task-quota-and-event-records-cleaned"
    ],
    dataPolicy: {
      syntheticDataOnly: true,
      customerDataAccessed: false,
      productionCredentialsAccessed: false,
      credentialsIncludedInEvidence: false
    },
    limitations: [
      "local-docker-mysql-integration-not-customer-environment",
      "does-not-replace-representative-old-agent-binary-upgrade-test",
      "does-not-replace-customer-offline-upgrade-and-rollback-validation"
    ]
  }' >"$temporary/report.json"
install -m 0600 "$temporary/report.json" "$report_file"

echo "通过: Local Agent协议2-3兼容窗口、协议3 Task Bundle门禁和协议1/4安全升级路径"
echo "证据: $report_file"
