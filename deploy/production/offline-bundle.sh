#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 3 ]] || { echo "用法: $0 VERSION RELEASE_SECURITY_DIRECTORY OUTPUT.tar" >&2; exit 1; }
version=$1
security_evidence=$2
output=$3
registry=${APPFORGE_IMAGE_REGISTRY:?required}
platform=${APPFORGE_OFFLINE_PLATFORM:-linux/amd64}
licensectl=${APPFORGE_LICENSECTL_BINARY:?required}
appforgectl=${APPFORGE_APPFORGECTL_BINARY:?required}
cosign=${APPFORGE_COSIGN_BINARY:?required}
docker_bin=${APPFORGE_DOCKER_BINARY:-}
if [[ -z $docker_bin ]]; then
  docker_bin=$(command -v docker || true)
fi
certificate_identity=${APPFORGE_RELEASE_CERTIFICATE_IDENTITY:?required}
certificate_issuer=${APPFORGE_RELEASE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}
case "$platform" in
  linux/amd64|linux/arm64) ;;
  *) echo "离线介质目标平台只允许 linux/amd64 或 linux/arm64" >&2; exit 1 ;;
esac
for tool in "$licensectl" "$appforgectl"; do
  [[ -f $tool && ! -L $tool && -x $tool ]] || { echo "交付工具必须是可执行普通文件: $tool" >&2; exit 1; }
done
[[ -f $cosign && ! -L $cosign && -x $cosign ]] || { echo "Cosign 必须是可执行普通文件: $cosign" >&2; exit 1; }
[[ -f $docker_bin && ! -L $docker_bin && -x $docker_bin ]] || { echo "Docker 客户端必须是可执行普通文件: $docker_bin" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "缺少 jq" >&2; exit 1; }
delivery_dir=$(cd "$(dirname "$0")" && pwd)
repo_root=$(cd "$delivery_dir/../.." && pwd)
APPFORGE_REQUIRE_COSIGN_VERIFY=1 \
APPFORGE_COSIGN_BINARY="$cosign" \
APPFORGE_RELEASE_CERTIFICATE_IDENTITY="$certificate_identity" \
APPFORGE_RELEASE_OIDC_ISSUER="$certificate_issuer" \
  "$delivery_dir/validate-release-evidence.sh" "$security_evidence" "$version" "$registry"
images=(system core builder builder-worker api admin-ui agent-ui local-agent egress-proxy mysql etcd minio minio-mc etcd-init migrate mysql-binlog-tools)
refs=()
platform_refs=()
signed_refs=()
platform_os=${platform%/*}
platform_arch=${platform#*/}

resolve_platform_ref() {
  local signed_ref=$1 raw platform_digest repository
  raw=$("$docker_bin" buildx imagetools inspect "$signed_ref" --raw)
  if jq -e '.manifests | type == "array"' >/dev/null 2>&1 <<<"$raw"; then
    platform_digest=$(jq -r --arg os "$platform_os" --arg architecture "$platform_arch" \
      '[.manifests[] | select(.platform.os == $os and .platform.architecture == $architecture)][0].digest // empty' \
      <<<"$raw")
    [[ $platform_digest =~ ^sha256:[0-9a-f]{64}$ ]] || {
      echo "签名镜像索引不包含目标平台 $platform: $signed_ref" >&2
      return 1
    }
    repository=${signed_ref%@*}
    printf '%s@%s\n' "$repository" "$platform_digest"
    return
  fi
  printf '%s\n' "$signed_ref"
}

for image in "${images[@]}"; do
  digest_ref=$(tr -d '\r\n' <"$security_evidence/$image.image.txt")
  platform_ref=$(resolve_platform_ref "$digest_ref")
  tag_ref="$registry/$image:$version"
  "$docker_bin" pull --platform "$platform" "$platform_ref"
  "$docker_bin" tag "$platform_ref" "$tag_ref"
  refs+=("$tag_ref")
  platform_refs+=("$platform_ref")
  signed_refs+=("$digest_ref")
done
third_party_components=(redis alpine)
third_party_tags=(redis:7.4-alpine alpine:3.22)
for index in "${!third_party_components[@]}"; do
  component=${third_party_components[$index]}
  tag_ref=${third_party_tags[$index]}
  digest_ref=$(tr -d '\r\n' <"$security_evidence/third-party-$component.image.txt")
  platform_ref=$(resolve_platform_ref "$digest_ref")
  "$docker_bin" pull --platform "$platform" "$platform_ref"
  "$docker_bin" tag "$platform_ref" "$tag_ref"
  refs+=("$tag_ref")
  platform_refs+=("$platform_ref")
  signed_refs+=("$digest_ref")
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
"$docker_bin" save "${refs[@]}" -o "$tmp/images.tar"
for file in docker-compose.yml .env.example egress-allowlist.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh archive-binlogs.sh pitr-restore.sh configure-object-replication.sh diagnostics.sh; do
  cp "$delivery_dir/$file" "$tmp/$file"
done
cp "$repo_root/deploy/etcd/admin-api.yaml" "$tmp/admin-api.yaml.template"
cp -R "$repo_root/deploy/local-agent" "$tmp/local-agent"
mkdir -p "$tmp/docs"
mkdir -p "$tmp/bin"
mkdir -p "$tmp/security"
cp "$licensectl" "$tmp/bin/licensectl"
cp "$appforgectl" "$tmp/bin/appforgectl"
cp "$delivery_dir/validate-release-evidence.sh" "$tmp/bin/validate-release-evidence"
cp "$delivery_dir/init-customer-site-evidence.sh" "$tmp/bin/init-customer-site-evidence"
cp "$delivery_dir/assemble-customer-site-evidence.sh" "$tmp/bin/assemble-customer-site-evidence"
cp "$delivery_dir/validate-customer-site-evidence.sh" "$tmp/bin/validate-customer-site-evidence"
cp "$security_evidence"/* "$tmp/security/"
chmod 0755 "$tmp/bin/licensectl" "$tmp/bin/appforgectl" "$tmp/bin/validate-release-evidence" \
  "$tmp/bin/init-customer-site-evidence" "$tmp/bin/assemble-customer-site-evidence" "$tmp/bin/validate-customer-site-evidence"
for file in delivery-guide.md local-agent-artifact-protocol.md customer-storage-contract.md air-gapped-artifact-contract.md remote-apk-signing-contract.md restricted-egress-proxy.md secret-providers.md version-compatibility.md customer-site-acceptance.md; do
  cp "$repo_root/docs/enterprise/$file" "$tmp/docs/$file"
done
printf '%s\n' "${refs[@]}" > "$tmp/IMAGES"
printf '%s\n' "$platform" > "$tmp/PLATFORM"
for index in "${!refs[@]}"; do
  printf '%s|%s|%s\n' "${refs[$index]}" "${platform_refs[$index]}" "${signed_refs[$index]}"
done > "$tmp/PLATFORM-IMAGES"
(cd "$tmp" && {
  sha256sum images.tar docker-compose.yml .env.example egress-allowlist.example nginx.conf admin-api.yaml.template preflight.sh render-config.sh backup.sh restore.sh archive-binlogs.sh pitr-restore.sh configure-object-replication.sh diagnostics.sh IMAGES PLATFORM PLATFORM-IMAGES
  find local-agent docs bin security -type f | LC_ALL=C sort | while IFS= read -r file; do sha256sum "$file"; done
} > SHA256SUMS)
tar -C "$tmp" -cf "$output" .
echo "含签名发布安全证据的离线 OCI 交付包已生成: $output"
