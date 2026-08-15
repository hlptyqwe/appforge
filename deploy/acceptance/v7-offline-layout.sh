#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
package_root="$temporary/package"
install_root="$temporary/install"
fixture_image=${APPFORGE_OFFLINE_ACCEPTANCE_IMAGE:-appforge-dev-etcd-init:latest}

grep -q 'minio/mc:RELEASE.2025-04-16T18-13-26Z' "$repo_root/deploy/production/offline-bundle.sh" || {
  echo "验收失败: 离线包未包含MinIO Bucket初始化镜像" >&2
  exit 1
}
mkdir -p "$package_root/docs" "$package_root/bin"

for file in docker-compose.yml .env.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh diagnostics.sh; do
  cp "$repo_root/deploy/production/$file" "$package_root/$file"
done
cp -R "$repo_root/deploy/local-agent" "$package_root/local-agent"
for file in delivery-guide.md local-agent-artifact-protocol.md secret-providers.md version-compatibility.md; do
  cp "$repo_root/docs/enterprise/$file" "$package_root/docs/$file"
done
cp "$repo_root/deploy/production/preflight.sh" "$package_root/bin/licensectl"
cp "$repo_root/deploy/production/preflight.sh" "$package_root/bin/appforgectl"
chmod 0755 "$package_root/bin/licensectl" "$package_root/bin/appforgectl"
printf '%s\n' "$fixture_image" >"$package_root/IMAGES"
docker image inspect "$fixture_image" >/dev/null
docker save "$fixture_image" -o "$package_root/images.tar"
(cd "$package_root" && {
  sha256sum images.tar docker-compose.yml .env.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh diagnostics.sh IMAGES
  find local-agent docs bin -type f | LC_ALL=C sort | while IFS= read -r file; do sha256sum "$file"; done
} >SHA256SUMS)
tar -C "$package_root" -cf "$temporary/offline.tar" .

"$repo_root/deploy/production/offline-install.sh" "$temporary/offline.tar" "$install_root" >/dev/null
for required in diagnostics.sh local-agent/docker-compose.yml local-agent/register.sh local-agent/secret-import.sh \
  local-agent/upgrade.sh docs/delivery-guide.md docs/version-compatibility.md; do
  [[ -f "$install_root/$required" ]] || { echo "验收失败: 离线安装缺少 $required" >&2; exit 1; }
done
[[ -x "$install_root/diagnostics.sh" && -x "$install_root/local-agent/register.sh" ]] || {
  echo "验收失败: 离线安装脚本没有可执行权限" >&2; exit 1;
}
[[ -x "$install_root/bin/licensectl" && -x "$install_root/bin/appforgectl" ]] || {
  echo "验收失败: 离线安装缺少可执行交付CLI" >&2; exit 1;
}
echo "通过: 离线包SHA校验、OCI导入、控制面文件、CLI、Local Agent安装包和企业文档落盘"
