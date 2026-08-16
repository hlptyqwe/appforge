#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 6 ]] || {
  echo "用法: $0 BASE_EVIDENCE BASE_VERSION TARGET_EVIDENCE TARGET_VERSION IMAGE_REGISTRY REPORT.json" >&2
  exit 1
}

base_evidence=$1
base_version=$2
target_evidence=$3
target_version=$4
registry=${5%/}
report_file=$6
cosign=${APPFORGE_COSIGN_BINARY:?APPFORGE_COSIGN_BINARY required}
base_certificate_identity=${APPFORGE_BASE_RELEASE_CERTIFICATE_IDENTITY:?APPFORGE_BASE_RELEASE_CERTIFICATE_IDENTITY required}
target_certificate_identity=${APPFORGE_TARGET_RELEASE_CERTIFICATE_IDENTITY:?APPFORGE_TARGET_RELEASE_CERTIFICATE_IDENTITY required}
certificate_issuer=${APPFORGE_RELEASE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}
node_image=${APPFORGE_V7_KIND_NODE_IMAGE:-kindest/node:v1.32.2}
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
components=(system core builder builder-worker api admin-ui agent-ui etcd-init migrate)
dependency_components=(mysql etcd minio)

for evidence in "$base_evidence" "$target_evidence"; do
  [[ -d $evidence && ! -L $evidence ]] || { echo "正式发布证据必须是普通目录: $evidence" >&2; exit 1; }
done
for version in "$base_version" "$target_version"; do
  [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "版本必须是语义化版本: $version" >&2; exit 1; }
done
[[ $base_version != "$target_version" ]] || { echo "基础版本和目标版本不能相同" >&2; exit 1; }
[[ $registry =~ ^[A-Za-z0-9._-]+([:/][A-Za-z0-9._-]+)+$ ]] || { echo "镜像仓库不合法" >&2; exit 1; }
[[ -f $cosign && ! -L $cosign && -x $cosign ]] || { echo "Cosign 必须是可执行普通文件" >&2; exit 1; }
for command_path in docker jq sha256sum; do
  command -v "$command_path" >/dev/null || { echo "缺少命令: $command_path" >&2; exit 1; }
done

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

temporary=$(mktemp -d /tmp/appforge-v7-formal-kubernetes.XXXXXX)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
runtime_report="$temporary/runtime.json"

validate_evidence() {
  local evidence=$1 version=$2 identity=$3
  APPFORGE_REQUIRE_COSIGN_VERIFY=1 \
  APPFORGE_COSIGN_BINARY="$cosign" \
  APPFORGE_RELEASE_CERTIFICATE_IDENTITY="$identity" \
  APPFORGE_RELEASE_OIDC_ISSUER="$certificate_issuer" \
    "$repo_root/deploy/production/validate-release-evidence.sh" "$evidence" "$version" "$registry" >/dev/null
}

validate_evidence "$base_evidence" "$base_version" "$base_certificate_identity"
validate_evidence "$target_evidence" "$target_version" "$target_certificate_identity"

base_commit=$(jq -r '.gitCommit // empty' "$base_evidence/RELEASE-MANIFEST.json")
target_commit=$(jq -r '.gitCommit // empty' "$target_evidence/RELEASE-MANIFEST.json")
[[ $base_commit =~ ^[0-9a-f]{40}$ && $target_commit =~ ^[0-9a-f]{40}$ ]] || {
  echo "发布清单缺少合法 Git commit" >&2
  exit 1
}
[[ $base_commit != "$target_commit" ]] || { echo "正式 Kubernetes 升级拒绝相同 Git commit" >&2; exit 1; }

pull_signed_component() {
  local evidence=$1 version=$2 identity=$3 component=$4 signed_ref
  signed_ref=$(tr -d '\r\n' <"$evidence/$component.image.txt")
  "$cosign" verify "$signed_ref" \
    --certificate-identity "$identity" \
    --certificate-oidc-issuer "$certificate_issuer" >/dev/null
  docker pull --platform linux/amd64 "$signed_ref" >/dev/null
  docker tag "$signed_ref" "$registry/$component:$version"
}

for component in "${components[@]}" "${dependency_components[@]}"; do
  pull_signed_component "$base_evidence" "$base_version" "$base_certificate_identity" "$component"
done
for component in "${components[@]}"; do
  pull_signed_component "$target_evidence" "$target_version" "$target_certificate_identity" "$component"
done

redis_ref=$(tr -d '\r\n' <"$base_evidence/third-party-redis.image.txt")
alpine_ref=$(tr -d '\r\n' <"$base_evidence/third-party-alpine.image.txt")
docker pull --platform linux/amd64 "$redis_ref" >/dev/null
docker tag "$redis_ref" redis:7.4-alpine
docker pull --platform linux/amd64 "$alpine_ref" >/dev/null
docker tag "$alpine_ref" alpine:3.22
docker pull --platform linux/amd64 "$node_image" >/dev/null
node_image_digest=$(docker image inspect "$node_image" --format '{{index .RepoDigests 0}}')

DOCKER_DEFAULT_PLATFORM=linux/amd64 \
APPFORGE_V7_KUBERNETES_FORMAL_RELEASE=1 \
APPFORGE_V7_KUBERNETES_REGISTRY="$registry" \
APPFORGE_V7_KUBERNETES_OLD_VERSION="$base_version" \
APPFORGE_V7_KUBERNETES_NEW_VERSION="$target_version" \
APPFORGE_V7_KIND_NODE_IMAGE="$node_image" \
APPFORGE_V7_KIND_NODE_ARCHITECTURE=amd64 \
APPFORGE_V7_KUBERNETES_REPORT_FILE="$runtime_report" \
  "$repo_root/deploy/acceptance/v7-kubernetes-upgrade.sh"

base_evidence_sha256=$(sha256sum "$base_evidence/SHA256SUMS" | awk '{print $1}')
target_evidence_sha256=$(sha256sum "$target_evidence/SHA256SUMS" | awk '{print $1}')
accepted_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p "$(dirname "$report_file")"
umask 077
jq -n \
  --arg acceptedAt "$accepted_at" \
  --arg registry "$registry" \
  --arg baseVersion "$base_version" \
  --arg targetVersion "$target_version" \
  --arg baseCommit "$base_commit" \
  --arg targetCommit "$target_commit" \
  --arg baseCertificateIdentity "$base_certificate_identity" \
  --arg targetCertificateIdentity "$target_certificate_identity" \
  --arg certificateIssuer "$certificate_issuer" \
  --arg baseEvidenceSha256 "$base_evidence_sha256" \
  --arg targetEvidenceSha256 "$target_evidence_sha256" \
  --arg redisRef "$redis_ref" \
  --arg alpineRef "$alpine_ref" \
  --arg nodeImageDigest "$node_image_digest" \
  --slurpfile runtime "$runtime_report" \
  '{
    schemaVersion: 1,
    kind: "appforge-v7-formal-kubernetes-upgrade-acceptance",
    acceptedAt: $acceptedAt,
    result: "passed",
    registry: $registry,
    platform: "linux/amd64",
    upgradePath: {
      baseVersion: $baseVersion,
      targetVersion: $targetVersion,
      rollbackVersion: $baseVersion,
      baseGitCommit: $baseCommit,
      targetGitCommit: $targetCommit,
      commitsDifferent: true
    },
    releaseEvidence: {
      aggregateSigstoreVerified: true,
      liveCosignVerifiedImageCount: 21,
      baseCertificateIdentity: $baseCertificateIdentity,
      targetCertificateIdentity: $targetCertificateIdentity,
      certificateIssuer: $certificateIssuer,
      baseSha256SumsFileSha256: $baseEvidenceSha256,
      targetSha256SumsFileSha256: $targetEvidenceSha256
    },
    runtimeInputs: {
      redisDigestRef: $redisRef,
      alpineDigestRef: $alpineRef,
      kindNodeImageDigest: $nodeImageDigest
    },
    runtime: $runtime[0],
    boundary: "使用公开正式 tag 的签名 linux/amd64 镜像和原生 amd64 kind 完成滚动升级与应用回滚；不替代客户目标 Kubernetes、CSI、Ingress、私有镜像仓库和现场交付验收。"
  }' >"$report_file"
chmod 0600 "$report_file"

echo "通过: 正式签名 Kubernetes 镜像 ${base_version}->${target_version}->${base_version} 原生 amd64 滚动升级与回滚"
echo "证据: $report_file"
