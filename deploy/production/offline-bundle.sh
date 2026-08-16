#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 3 ]] || { echo "用法: $0 VERSION RELEASE_SECURITY_DIRECTORY OUTPUT.tar" >&2; exit 1; }
version=$1
security_evidence=$2
output=$3
registry=${APPFORGE_IMAGE_REGISTRY:?required}
licensectl=${APPFORGE_LICENSECTL_BINARY:?required}
appforgectl=${APPFORGE_APPFORGECTL_BINARY:?required}
cosign=${APPFORGE_COSIGN_BINARY:?required}
docker_bin=${APPFORGE_DOCKER_BINARY:-}
if [[ -z $docker_bin ]]; then
  docker_bin=$(command -v docker || true)
fi
certificate_identity=${APPFORGE_RELEASE_CERTIFICATE_IDENTITY:?required}
certificate_issuer=${APPFORGE_RELEASE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}
for tool in "$licensectl" "$appforgectl"; do
  [[ -f $tool && ! -L $tool && -x $tool ]] || { echo "交付工具必须是可执行普通文件: $tool" >&2; exit 1; }
done
[[ -f $cosign && ! -L $cosign && -x $cosign ]] || { echo "Cosign 必须是可执行普通文件: $cosign" >&2; exit 1; }
[[ -f $docker_bin && ! -L $docker_bin && -x $docker_bin ]] || { echo "Docker 客户端必须是可执行普通文件: $docker_bin" >&2; exit 1; }
delivery_dir=$(cd "$(dirname "$0")" && pwd)
repo_root=$(cd "$delivery_dir/../.." && pwd)
APPFORGE_REQUIRE_COSIGN_VERIFY=1 \
APPFORGE_COSIGN_BINARY="$cosign" \
APPFORGE_RELEASE_CERTIFICATE_IDENTITY="$certificate_identity" \
APPFORGE_RELEASE_OIDC_ISSUER="$certificate_issuer" \
  "$delivery_dir/validate-release-evidence.sh" "$security_evidence" "$version" "$registry"
images=(system core builder builder-worker api admin-ui agent-ui local-agent egress-proxy mysql etcd minio minio-mc etcd-init migrate mysql-binlog-tools)
refs=()
for image in "${images[@]}"; do
  digest_ref=$(tr -d '\r\n' <"$security_evidence/$image.image.txt")
  tag_ref="$registry/$image:$version"
  "$docker_bin" pull "$digest_ref"
  "$docker_bin" tag "$digest_ref" "$tag_ref"
  refs+=("$tag_ref")
done
third_party_components=(redis alpine)
third_party_tags=(redis:7.4-alpine alpine:3.22)
for index in "${!third_party_components[@]}"; do
  component=${third_party_components[$index]}
  tag_ref=${third_party_tags[$index]}
  digest_ref=$(tr -d '\r\n' <"$security_evidence/third-party-$component.image.txt")
  "$docker_bin" pull "$digest_ref"
  "$docker_bin" tag "$digest_ref" "$tag_ref"
  refs+=("$tag_ref")
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
"$docker_bin" save "${refs[@]}" -o "$tmp/images.tar"
for file in docker-compose.yml .env.example egress-allowlist.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh archive-binlogs.sh pitr-restore.sh configure-object-replication.sh diagnostics.sh; do
  cp "$delivery_dir/$file" "$tmp/$file"
done
cp -R "$repo_root/deploy/local-agent" "$tmp/local-agent"
mkdir -p "$tmp/docs"
mkdir -p "$tmp/bin"
mkdir -p "$tmp/security"
cp "$licensectl" "$tmp/bin/licensectl"
cp "$appforgectl" "$tmp/bin/appforgectl"
cp "$delivery_dir/validate-release-evidence.sh" "$tmp/bin/validate-release-evidence"
cp "$security_evidence"/* "$tmp/security/"
chmod 0755 "$tmp/bin/licensectl" "$tmp/bin/appforgectl" "$tmp/bin/validate-release-evidence"
for file in delivery-guide.md local-agent-artifact-protocol.md customer-storage-contract.md air-gapped-artifact-contract.md remote-apk-signing-contract.md restricted-egress-proxy.md secret-providers.md version-compatibility.md; do
  cp "$repo_root/docs/enterprise/$file" "$tmp/docs/$file"
done
printf '%s\n' "${refs[@]}" > "$tmp/IMAGES"
(cd "$tmp" && {
  sha256sum images.tar docker-compose.yml .env.example egress-allowlist.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh archive-binlogs.sh pitr-restore.sh configure-object-replication.sh diagnostics.sh IMAGES
  find local-agent docs bin security -type f | LC_ALL=C sort | while IFS= read -r file; do sha256sum "$file"; done
} > SHA256SUMS)
tar -C "$tmp" -cf "$output" .
echo "含签名发布安全证据的离线 OCI 交付包已生成: $output"
