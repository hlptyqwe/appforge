#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
registry=${APPFORGE_OFFLINE_INSTALL_REGISTRY:-appforge-private-acceptance}
version=${APPFORGE_OFFLINE_INSTALL_VERSION:-1.2.0}
temporary_migrate_image="$registry/migrate:${version}-v113-offline-$$"
report_file=${APPFORGE_OFFLINE_INSTALL_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-offline-install-20260815.json}
upgrade_mode=${APPFORGE_OFFLINE_INSTALL_UPGRADE_MODE:-0}
upgrade_version=${APPFORGE_OFFLINE_INSTALL_UPGRADE_VERSION:-1.2.1}

[[ $temporary_migrate_image =~ ^[A-Za-z0-9._:/-]+$ ]]
docker image inspect mysql:8.4 >/dev/null || { echo "验收失败: 缺少预载镜像 mysql:8.4" >&2; exit 1; }

cleanup() {
  docker image rm "$temporary_migrate_image" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker build --pull=false \
  --build-arg APPFORGE_SCHEMA_TARGET=20260815_113_v7_air_gapped \
  -f "$repo_root/deploy/docker/migrate.Dockerfile" \
  -t "$temporary_migrate_image" "$repo_root" >/dev/null

APPFORGE_PRIVATE_ACCEPTANCE_REGISTRY="$registry" \
APPFORGE_PRIVATE_ACCEPTANCE_VERSION="$version" \
APPFORGE_PRIVATE_ACCEPTANCE_OFFLINE_MODE=1 \
APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_MODE="$upgrade_mode" \
APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_VERSION="$upgrade_version" \
APPFORGE_PRIVATE_ACCEPTANCE_MIGRATE_IMAGE="$temporary_migrate_image" \
APPFORGE_PRIVATE_ACCEPTANCE_REPORT_FILE="$report_file" \
  "$repo_root/deploy/acceptance/v7-private-install.sh"

echo "证据: $report_file"
if [[ $upgrade_mode == 1 ]]; then
  echo "边界: 本地模拟断网应用升级/回滚，不替代客户物理断网、真实版本差异、Schema升级或正式发布介质验收"
else
  echo "边界: 本地模拟断网全新安装，不替代客户物理断网安装、离线升级或正式发布介质验收"
fi
