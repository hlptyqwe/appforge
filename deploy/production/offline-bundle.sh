#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 2 ]] || { echo "用法: $0 VERSION OUTPUT.tar" >&2; exit 1; }
version=$1
output=$2
registry=${APPFORGE_IMAGE_REGISTRY:?required}
licensectl=${APPFORGE_LICENSECTL_BINARY:?required}
appforgectl=${APPFORGE_APPFORGECTL_BINARY:?required}
for tool in "$licensectl" "$appforgectl"; do
  [[ -f $tool && ! -L $tool && -x $tool ]] || { echo "交付工具必须是可执行普通文件: $tool" >&2; exit 1; }
done
images=(system core builder builder-worker api admin-ui agent-ui local-agent etcd-init migrate)
refs=()
for image in "${images[@]}"; do
  ref="$registry/$image:$version"
  docker pull "$ref"
  refs+=("$ref")
done
third_party=(mysql:8.4 redis:7.4-alpine quay.io/coreos/etcd:v3.6.12 minio/minio:RELEASE.2025-04-22T22-12-26Z minio/mc:RELEASE.2025-04-16T18-13-26Z alpine:3.22)
for ref in "${third_party[@]}"; do
  docker pull "$ref"
  refs+=("$ref")
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
docker save "${refs[@]}" -o "$tmp/images.tar"
delivery_dir=$(cd "$(dirname "$0")" && pwd)
repo_root=$(cd "$delivery_dir/../.." && pwd)
for file in docker-compose.yml .env.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh diagnostics.sh; do
  cp "$delivery_dir/$file" "$tmp/$file"
done
cp -R "$repo_root/deploy/local-agent" "$tmp/local-agent"
mkdir -p "$tmp/docs"
mkdir -p "$tmp/bin"
cp "$licensectl" "$tmp/bin/licensectl"
cp "$appforgectl" "$tmp/bin/appforgectl"
chmod 0755 "$tmp/bin/licensectl" "$tmp/bin/appforgectl"
for file in delivery-guide.md local-agent-artifact-protocol.md secret-providers.md version-compatibility.md; do
  cp "$repo_root/docs/enterprise/$file" "$tmp/docs/$file"
done
printf '%s\n' "${refs[@]}" > "$tmp/IMAGES"
(cd "$tmp" && {
  sha256sum images.tar docker-compose.yml .env.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh diagnostics.sh IMAGES
  find local-agent docs bin -type f | LC_ALL=C sort | while IFS= read -r file; do sha256sum "$file"; done
} > SHA256SUMS)
tar -C "$tmp" -cf "$output" .
echo "离线 OCI 交付包已生成: $output"
