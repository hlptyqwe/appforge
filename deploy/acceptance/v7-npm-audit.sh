#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
ui_dir="$repo_root/appforge-ui"
report_file=${APPFORGE_NPM_AUDIT_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-npm-audit-20260816.json}
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT

(
  cd "$ui_dir"
  npm audit --omit=dev --audit-level=high --json >"$temporary/production.json"
  npm audit --audit-level=high --json >"$temporary/all.json"
)

for scope in production all; do
  jq -e '.metadata.vulnerabilities.total == 0' "$temporary/$scope.json" >/dev/null || {
    echo "验收失败: npm $scope 依赖仍存在漏洞" >&2
    exit 1
  }
done

mkdir -p "$(dirname "$report_file")"
umask 077
jq -n \
  --arg auditedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg nodeVersion "$(node --version)" \
  --arg npmVersion "$(npm --version)" \
  --arg viteVersion "$(cd "$ui_dir" && node -p 'require("./node_modules/vite/package.json").version')" \
  --arg pluginVueVersion "$(cd "$ui_dir" && node -p 'require("./node_modules/@vitejs/plugin-vue/package.json").version')" \
  '{
    schemaVersion: 1,
    kind: "appforge-v7-npm-official-audit",
    auditedAt: $auditedAt,
    registry: "npm-official-configured-registry",
    nodeVersion: $nodeVersion,
    npmVersion: $npmVersion,
    viteVersion: $viteVersion,
    pluginVueVersion: $pluginVueVersion,
    productionDependencies: {vulnerabilities: 0, result: "passed"},
    allDependencies: {vulnerabilities: 0, result: "passed"},
    dependencyDetailsExported: false
  }' >"$report_file"
chmod 0600 "$report_file"

echo "通过: npm 官方生产依赖与全部依赖审计均为0漏洞"
echo "证据: $report_file"
