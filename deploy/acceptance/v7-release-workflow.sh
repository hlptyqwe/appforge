#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
workflow="$repo_root/.github/workflows/release-security.yml"
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT

ruby -e 'require "yaml"; YAML.load_file(ARGV.fetch(0))' "$workflow" 2>/dev/null
grep -q 'docker/metadata-action@v5' "$workflow"
grep -q 'type=semver,pattern={{version}}' "$workflow"
grep -q 'steps.build.outputs.digest' "$workflow"
grep -q 'format: sarif' "$workflow"
grep -q 'cosign verify ' "$workflow"
grep -q 'cosign verify-blob' "$workflow"
grep -q 'licensectl-linux-' "$workflow"
grep -q 'appforgectl-linux-' "$workflow"
if grep -q 'workflow_dispatch' "$workflow"; then
  echo "验收失败: 发布工作流存在未受 tag 约束的手动发布入口" >&2
  exit 1
fi

for arch in amd64 arm64; do
  (
    cd "$repo_root/appforge-api"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOCACHE=/private/tmp/appforge-go-build-cache \
      go build -trimpath -ldflags="-s -w" -o "$temporary/licensectl-linux-$arch" ./cmd/licensectl
  )
  (
    cd "$repo_root/appforgectl"
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOCACHE=/private/tmp/appforge-go-build-cache \
      go build -trimpath -ldflags="-s -w" -o "$temporary/appforgectl-linux-$arch" .
  )
done
(cd "$temporary" && sha256sum appforgectl-linux-* licensectl-linux-* >SHA256SUMS)
[[ $(wc -l <"$temporary/SHA256SUMS" | tr -d ' ') == 4 ]]
echo "通过: tag限定发布、无v语义化镜像、digest扫描签名回验、证据归档和双架构CLI构建"
