#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
compose_file=${APPFORGE_DIAGNOSTIC_COMPOSE_FILE:-$repo_root/deploy/docker-compose.dev.yml}
api_url=${APPFORGE_DIAGNOSTIC_API_URL:-http://127.0.0.1:8888}
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT

APPFORGE_ENV_FILE=/dev/null \
APPFORGE_COMPOSE_FILE="$compose_file" \
APPFORGE_DIAGNOSTIC_API_URL="$api_url" \
APPFORGE_PUBLIC_ORIGIN="$api_url" \
APPFORGE_VERSION=acceptance \
APPFORGE_DEPLOYMENT_MODE=acceptance \
APPFORGE_MYSQL_PASSWORD=${APPFORGE_DIAGNOSTIC_MYSQL_PASSWORD:-appforge_dev_password} \
APPFORGE_MYSQL_ROOT_PASSWORD=${APPFORGE_DIAGNOSTIC_MYSQL_ROOT_PASSWORD:-appforge_dev_root_password} \
  "$repo_root/deploy/production/diagnostics.sh" "$temporary/diagnostics.tar.gz"

mode=$(stat -c %a "$temporary/diagnostics.tar.gz" 2>/dev/null || stat -f %Lp "$temporary/diagnostics.tar.gz")
[[ $mode == 600 ]] || { echo "验收失败: 诊断包权限不是0600" >&2; exit 1; }
mkdir "$temporary/extracted"
tar -C "$temporary/extracted" -xzf "$temporary/diagnostics.tar.gz"
(cd "$temporary/extracted" && sha256sum -c SHA256SUMS >/dev/null)
for required in metadata.json components.tsv healthz.json readyz.json schema-migrations.tsv local-agent-summary.tsv deployment-file-sha256.tsv; do
  [[ -f "$temporary/extracted/$required" ]] || { echo "验收失败: 诊断文件缺失: $required" >&2; exit 1; }
done
[[ -s "$temporary/extracted/components.tsv" && -s "$temporary/extracted/schema-migrations.tsv" ]] || {
  echo "验收失败: 组件或迁移诊断为空" >&2; exit 1;
}
if grep -R -E -q -- 'appforge_dev_password|appforge_dev_root_password|BEGIN .*PRIVATE KEY|keystorePassword|keyPassword' "$temporary/extracted"; then
  echo "验收失败: 诊断包包含敏感字段" >&2
  exit 1
fi
echo "通过: 诊断包最小采集、0600权限、SHA清单和敏感值扫描"
