#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 2 ]] || { echo "用法: $0 VERSION OUTPUT.tar" >&2; exit 1; }
version=$1
output=$2
registry=${APPFORGE_IMAGE_REGISTRY:?required}
images=(system core builder builder-worker api admin-ui agent-ui local-agent etcd-init migrate)
refs=()
for image in "${images[@]}"; do
  ref="$registry/$image:$version"
  docker pull "$ref"
  refs+=("$ref")
done
third_party=(mysql:8.4 redis:7.4-alpine quay.io/coreos/etcd:v3.6.12 minio/minio:RELEASE.2025-04-22T22-12-26Z alpine:3.22)
for ref in "${third_party[@]}"; do
  docker pull "$ref"
  refs+=("$ref")
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
docker save "${refs[@]}" -o "$tmp/images.tar"
delivery_dir=$(cd "$(dirname "$0")" && pwd)
for file in docker-compose.yml .env.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh; do
  cp "$delivery_dir/$file" "$tmp/$file"
done
printf '%s\n' "${refs[@]}" > "$tmp/IMAGES"
(cd "$tmp" && sha256sum images.tar docker-compose.yml .env.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh IMAGES > SHA256SUMS)
tar -C "$tmp" -cf "$output" .
echo "离线 OCI 交付包已生成: $output"
