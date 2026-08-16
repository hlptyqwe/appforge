#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 6 ]] || {
  echo "用法: $0 BASE_BUNDLE.tar BASE_VERSION TARGET_BUNDLE.tar TARGET_VERSION IMAGE_REGISTRY REPORT.json" >&2
  exit 1
}

base_bundle=$1
base_version=$2
target_bundle=$3
target_version=$4
registry=$5
report_file=$6
cosign=${APPFORGE_COSIGN_BINARY:?APPFORGE_COSIGN_BINARY required}
base_certificate_identity=${APPFORGE_BASE_RELEASE_CERTIFICATE_IDENTITY:?APPFORGE_BASE_RELEASE_CERTIFICATE_IDENTITY required}
target_certificate_identity=${APPFORGE_TARGET_RELEASE_CERTIFICATE_IDENTITY:?APPFORGE_TARGET_RELEASE_CERTIFICATE_IDENTITY required}
certificate_issuer=${APPFORGE_RELEASE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
release_components=(system core builder builder-worker api admin-ui agent-ui local-agent egress-proxy mysql etcd minio minio-mc etcd-init migrate mysql-binlog-tools)
required_distinct_components=(system core builder builder-worker api admin-ui agent-ui etcd-init migrate)

for bundle in "$base_bundle" "$target_bundle"; do
  [[ -f $bundle && ! -L $bundle ]] || { echo "正式离线介质必须是普通文件: $bundle" >&2; exit 1; }
done
for version in "$base_version" "$target_version"; do
  [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "版本必须是语义化版本: $version" >&2; exit 1; }
done
[[ $base_version != "$target_version" ]] || { echo "基础版本和目标版本不能相同" >&2; exit 1; }
[[ $registry =~ ^[A-Za-z0-9._-]+([:/][A-Za-z0-9._-]+)+$ ]] || { echo "镜像仓库不合法" >&2; exit 1; }
[[ -f $cosign && ! -L $cosign && -x $cosign ]] || { echo "Cosign 必须是可执行普通文件" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "缺少 jq" >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 || { echo "缺少 sha256sum" >&2; exit 1; }

version_greater_than() {
  local left=$1 right=$2 left_major left_minor left_patch right_major right_minor right_patch
  IFS=. read -r left_major left_minor left_patch <<<"$left"
  IFS=. read -r right_major right_minor right_patch <<<"$right"
  ((left_major > right_major)) ||
    ((left_major == right_major && left_minor > right_minor)) ||
    ((left_major == right_major && left_minor == right_minor && left_patch > right_patch))
}

version_greater_than "$target_version" "$base_version" || {
  echo "目标版本必须高于基础版本: $base_version -> $target_version" >&2
  exit 1
}

temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
base_install="$temporary/base"
target_install="$temporary/target"
runtime_report="$temporary/runtime.json"
differences_tsv="$temporary/image-differences.tsv"
differences_json="$temporary/image-differences.json"

"$repo_root/deploy/production/offline-install.sh" "$base_bundle" "$base_install" >/dev/null
"$repo_root/deploy/production/offline-install.sh" "$target_bundle" "$target_install" >/dev/null

validate_media() {
  local install_root=$1 version=$2 certificate_identity=$3 platform
  platform=$(tr -d '\r\n' <"$install_root/PLATFORM")
  case "$platform" in linux/amd64|linux/arm64) ;; *) echo "正式介质平台不合法: $platform" >&2; return 1;; esac
  [[ $(wc -l <"$install_root/IMAGES" | tr -d ' ') == 18 ]] || { echo "正式介质镜像清单必须是18项: $version" >&2; return 1; }
  [[ $(wc -l <"$install_root/PLATFORM-IMAGES" | tr -d ' ') == 18 ]] || { echo "正式介质平台摘要映射必须是18项: $version" >&2; return 1; }
  APPFORGE_REQUIRE_COSIGN_VERIFY=1 \
  APPFORGE_COSIGN_BINARY="$cosign" \
  APPFORGE_RELEASE_CERTIFICATE_IDENTITY="$certificate_identity" \
  APPFORGE_RELEASE_OIDC_ISSUER="$certificate_issuer" \
    "$install_root/bin/validate-release-evidence" \
      "$install_root/security" "$version" "$registry" >/dev/null
  "$install_root/render-config.sh" "$install_root/.env.example" >/dev/null
  [[ -s $install_root/runtime/admin-api.yaml ]] || { echo "正式介质无法独立渲染 Admin API 配置: $version" >&2; return 1; }
}

validate_media "$base_install" "$base_version" "$base_certificate_identity"
validate_media "$target_install" "$target_version" "$target_certificate_identity"
base_platform=$(tr -d '\r\n' <"$base_install/PLATFORM")
target_platform=$(tr -d '\r\n' <"$target_install/PLATFORM")
[[ $base_platform == "$target_platform" ]] || {
  echo "基础与目标介质平台不一致: $base_platform != $target_platform" >&2
  exit 1
}

base_commit=$(jq -r '.gitCommit // empty' "$base_install/security/RELEASE-MANIFEST.json")
target_commit=$(jq -r '.gitCommit // empty' "$target_install/security/RELEASE-MANIFEST.json")
[[ $base_commit =~ ^[0-9a-f]{40}$ && $target_commit =~ ^[0-9a-f]{40}$ ]] || {
  echo "发布清单缺少合法 Git commit" >&2
  exit 1
}
[[ $base_commit != "$target_commit" ]] || {
  echo "正式升级拒绝相同 Git commit: $base_commit" >&2
  exit 1
}

component_line() {
  local mapping_file=$1 tag=$2 count
  count=$(grep -Fc "$tag|" "$mapping_file")
  [[ $count == 1 ]] || { echo "平台摘要映射缺失或重复: $tag" >&2; return 1; }
  grep -F "$tag|" "$mapping_file"
}

: >"$differences_tsv"
distinct_count=0
for component in "${release_components[@]}"; do
  base_line=$(component_line "$base_install/PLATFORM-IMAGES" "$registry/$component:$base_version")
  target_line=$(component_line "$target_install/PLATFORM-IMAGES" "$registry/$component:$target_version")
  base_platform_ref=$(cut -d'|' -f2 <<<"$base_line")
  target_platform_ref=$(cut -d'|' -f2 <<<"$target_line")
  different=false
  if [[ $base_platform_ref != "$target_platform_ref" ]]; then
    different=true
    distinct_count=$((distinct_count + 1))
  fi
  printf '%s\t%s\t%s\t%s\n' "$component" "$base_platform_ref" "$target_platform_ref" "$different" >>"$differences_tsv"
done

for component in "${required_distinct_components[@]}"; do
  difference=$(awk -F '\t' -v component="$component" '$1 == component { print $4 }' "$differences_tsv")
  [[ $difference == true ]] || {
    echo "正式升级拒绝同平台摘要组件: $component" >&2
    exit 1
  }
done

jq -Rn '[inputs | select(length > 0) | split("\t") | {
  component: .[0],
  basePlatformRef: .[1],
  targetPlatformRef: .[2],
  different: (.[3] == "true")
}]' <"$differences_tsv" >"$differences_json"

port_base=$((30000 + RANDOM % 10000))
APPFORGE_PRIVATE_ACCEPTANCE_REGISTRY="$registry" \
APPFORGE_PRIVATE_ACCEPTANCE_VERSION="$base_version" \
APPFORGE_PRIVATE_ACCEPTANCE_OFFLINE_MODE=1 \
APPFORGE_PRIVATE_ACCEPTANCE_FORMAL_RELEASE_MEDIA=1 \
APPFORGE_PRIVATE_ACCEPTANCE_FORMAL_RELEASE_PLATFORM="$base_platform" \
APPFORGE_PRIVATE_ACCEPTANCE_FORMAL_RELEASE_UPGRADE=1 \
APPFORGE_PRIVATE_ACCEPTANCE_DELIVERY_SOURCE="$base_install" \
APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_DELIVERY_SOURCE="$target_install" \
APPFORGE_PRIVATE_ACCEPTANCE_DELIVERY_SOURCE_KIND=formal-offline-media \
APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_MODE=1 \
APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_VERSION="$target_version" \
APPFORGE_PRIVATE_ACCEPTANCE_MIGRATE_IMAGE="$registry/migrate:$base_version" \
APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_MIGRATE_IMAGE="$registry/migrate:$target_version" \
APPFORGE_PRIVATE_ACCEPTANCE_ADMIN_PORT="$port_base" \
APPFORGE_PRIVATE_ACCEPTANCE_AGENT_PORT="$((port_base + 1))" \
APPFORGE_PRIVATE_ACCEPTANCE_GATEWAY_PORT="$((port_base + 2))" \
APPFORGE_PRIVATE_ACCEPTANCE_REPORT_FILE="$runtime_report" \
  "$repo_root/deploy/acceptance/v7-private-install.sh" >/dev/null

base_bundle_sha256=$(sha256sum "$base_bundle" | awk '{print $1}')
target_bundle_sha256=$(sha256sum "$target_bundle" | awk '{print $1}')
base_bundle_size=$(wc -c <"$base_bundle" | tr -d ' ')
target_bundle_size=$(wc -c <"$target_bundle" | tr -d ' ')
accepted_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p "$(dirname "$report_file")"
umask 077
jq -n \
  --arg acceptedAt "$accepted_at" \
  --arg registry "$registry" \
  --arg platform "$base_platform" \
  --arg baseVersion "$base_version" \
  --arg targetVersion "$target_version" \
  --arg baseCommit "$base_commit" \
  --arg targetCommit "$target_commit" \
  --arg baseBundleSha256 "$base_bundle_sha256" \
  --arg targetBundleSha256 "$target_bundle_sha256" \
  --argjson baseBundleSize "$base_bundle_size" \
  --argjson targetBundleSize "$target_bundle_size" \
  --arg baseCertificateIdentity "$base_certificate_identity" \
  --arg targetCertificateIdentity "$target_certificate_identity" \
  --arg certificateIssuer "$certificate_issuer" \
  --argjson comparedImageCount "${#release_components[@]}" \
  --argjson distinctImageCount "$distinct_count" \
  --slurpfile imageDifferences "$differences_json" \
  --slurpfile runtime "$runtime_report" \
  '{
    schemaVersion: 1,
    kind: "appforge-v7-formal-offline-upgrade-acceptance",
    acceptedAt: $acceptedAt,
    result: "passed",
    registry: $registry,
    platform: $platform,
    upgradePath: {
      baseVersion: $baseVersion,
      targetVersion: $targetVersion,
      rollbackVersion: $baseVersion,
      baseGitCommit: $baseCommit,
      targetGitCommit: $targetCommit,
      commitsDifferent: true,
      comparedImageCount: $comparedImageCount,
      distinctImageCount: $distinctImageCount,
      imageDifferences: $imageDifferences[0]
    },
    media: {
      base: {
        sizeBytes: $baseBundleSize,
        sha256: $baseBundleSha256,
        imageCount: 18,
        certificateIdentity: $baseCertificateIdentity
      },
      target: {
        sizeBytes: $targetBundleSize,
        sha256: $targetBundleSha256,
        imageCount: 18,
        certificateIdentity: $targetCertificateIdentity
      },
      certificateIssuer: $certificateIssuer,
      checksumVerifiedBeforeLoad: true,
      cosignVerified: true,
      deploymentFilesSwitchedAndRolledBack: true
    },
    runtime: $runtime[0],
    boundary: "使用两个公开正式 tag 的签名 linux/amd64 离线介质和不同 Git commit/平台镜像摘要，在本地 Docker internal 网络完成旧版到新版再回旧版；不替代客户物理断网介质交接、客户硬件/证书/KMS、生产数据或客户现场升级验收。"
  }' >"$report_file"
chmod 0600 "$report_file"

echo "通过: 两个正式签名版本 ${base_version}->${target_version}->${base_version} 本地断网升级/回滚，${distinct_count}/${#release_components[@]} 个发布镜像平台摘要不同"
echo "证据: $report_file"
