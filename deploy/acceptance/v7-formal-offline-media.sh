#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 4 ]] || {
  echo "用法: $0 BUNDLE.tar VERSION IMAGE_REGISTRY REPORT.json" >&2
  exit 1
}

bundle=$1
version=$2
registry=$3
report_file=$4
cosign=${APPFORGE_COSIGN_BINARY:?APPFORGE_COSIGN_BINARY required}
certificate_identity=${APPFORGE_RELEASE_CERTIFICATE_IDENTITY:?APPFORGE_RELEASE_CERTIFICATE_IDENTITY required}
certificate_issuer=${APPFORGE_RELEASE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

[[ -f $bundle && ! -L $bundle ]] || { echo "正式离线介质必须是普通文件" >&2; exit 1; }
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "版本必须是语义化版本" >&2; exit 1; }
[[ $registry =~ ^[A-Za-z0-9._-]+([:/][A-Za-z0-9._-]+)+$ ]] || { echo "镜像仓库不合法" >&2; exit 1; }
[[ -f $cosign && ! -L $cosign && -x $cosign ]] || { echo "Cosign 必须是可执行普通文件" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "缺少 jq" >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 || { echo "缺少 sha256sum" >&2; exit 1; }

temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
install_root="$temporary/install"
runtime_report="$temporary/runtime.json"

"$repo_root/deploy/production/offline-install.sh" "$bundle" "$install_root" >/dev/null

platform=$(tr -d '\r\n' <"$install_root/PLATFORM")
case "$platform" in linux/amd64|linux/arm64) ;; *) echo "正式介质平台不合法: $platform" >&2; exit 1;; esac
[[ $(wc -l <"$install_root/IMAGES" | tr -d ' ') == 18 ]] || { echo "正式介质镜像清单必须是18项" >&2; exit 1; }
[[ $(wc -l <"$install_root/PLATFORM-IMAGES" | tr -d ' ') == 18 ]] || { echo "正式介质平台摘要映射必须是18项" >&2; exit 1; }
while IFS='|' read -r tag_ref platform_ref signed_ref extra; do
  [[ -z $extra ]] || { echo "正式介质平台摘要映射字段过多" >&2; exit 1; }
  [[ $tag_ref =~ ^[A-Za-z0-9._:/-]+$ ]] || { echo "正式介质部署标签不合法" >&2; exit 1; }
  [[ $platform_ref =~ @sha256:[0-9a-f]{64}$ ]] || { echo "正式介质平台摘要不合法" >&2; exit 1; }
  [[ $signed_ref =~ @sha256:[0-9a-f]{64}$ ]] || { echo "正式介质签名索引摘要不合法" >&2; exit 1; }
done <"$install_root/PLATFORM-IMAGES"

APPFORGE_REQUIRE_COSIGN_VERIFY=1 \
APPFORGE_COSIGN_BINARY="$cosign" \
APPFORGE_RELEASE_CERTIFICATE_IDENTITY="$certificate_identity" \
APPFORGE_RELEASE_OIDC_ISSUER="$certificate_issuer" \
  "$install_root/bin/validate-release-evidence" \
    "$install_root/security" "$version" "$registry" >/dev/null

"$install_root/render-config.sh" "$install_root/.env.example" >/dev/null
[[ -s $install_root/runtime/admin-api.yaml ]] || { echo "正式介质无法独立渲染 Admin API 配置" >&2; exit 1; }

port_base=$((30000 + RANDOM % 10000))
APPFORGE_PRIVATE_ACCEPTANCE_REGISTRY="$registry" \
APPFORGE_PRIVATE_ACCEPTANCE_VERSION="$version" \
APPFORGE_PRIVATE_ACCEPTANCE_OFFLINE_MODE=1 \
APPFORGE_PRIVATE_ACCEPTANCE_FORMAL_RELEASE_MEDIA=1 \
APPFORGE_PRIVATE_ACCEPTANCE_FORMAL_RELEASE_PLATFORM="$platform" \
APPFORGE_PRIVATE_ACCEPTANCE_MIGRATE_IMAGE="$registry/migrate:$version" \
APPFORGE_PRIVATE_ACCEPTANCE_ADMIN_PORT="$port_base" \
APPFORGE_PRIVATE_ACCEPTANCE_AGENT_PORT="$((port_base + 1))" \
APPFORGE_PRIVATE_ACCEPTANCE_GATEWAY_PORT="$((port_base + 2))" \
APPFORGE_PRIVATE_ACCEPTANCE_REPORT_FILE="$runtime_report" \
  "$repo_root/deploy/acceptance/v7-private-install.sh" >/dev/null

bundle_sha256=$(sha256sum "$bundle" | awk '{print $1}')
bundle_size=$(wc -c <"$bundle" | tr -d ' ')
accepted_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p "$(dirname "$report_file")"
umask 077
jq -n \
  --arg acceptedAt "$accepted_at" \
  --arg version "$version" \
  --arg registry "$registry" \
  --arg platform "$platform" \
  --arg bundleSha256 "$bundle_sha256" \
  --argjson bundleSize "$bundle_size" \
  --arg certificateIdentity "$certificate_identity" \
  --arg certificateIssuer "$certificate_issuer" \
  --slurpfile runtime "$runtime_report" \
  '{
    schemaVersion: 1,
    kind: "appforge-v7-formal-offline-media-acceptance",
    acceptedAt: $acceptedAt,
    result: "passed",
    version: $version,
    registry: $registry,
    platform: $platform,
    bundle: {
      sizeBytes: $bundleSize,
      sha256: $bundleSha256,
      imageCount: 18,
      platformDigestMappingCount: 18,
      checksumVerifiedBeforeLoad: true,
      dockerLoadCompleted: true,
      selfContainedAdminApiTemplate: true
    },
    releaseEvidence: {
      cosignVerified: true,
      certificateIdentity: $certificateIdentity,
      certificateIssuer: $certificateIssuer
    },
    runtime: $runtime[0],
    boundary: "使用正式签名镜像和合成临时凭据在本地 Docker internal 网络完成离线安装；不替代客户物理断网、正式介质交接、客户证书/KMS、生产数据或客户硬件验收。"
  }' >"$report_file"
chmod 0600 "$report_file"

echo "通过: 正式签名离线介质 SHA、18镜像加载、独立配置渲染和本地断网全新安装"
echo "证据: $report_file"
