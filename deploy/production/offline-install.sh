#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 2 ]] || { echo "用法: $0 BUNDLE.tar INSTALL_DIRECTORY" >&2; exit 1; }
bundle=$1
install_dir=$2
[[ -f "$bundle" ]] || { echo "离线包不存在: $bundle" >&2; exit 1; }
if [[ -e "$install_dir" ]] && [[ -n $(find "$install_dir" -mindepth 1 -maxdepth 1 -print -quit) ]]; then
  echo "安装目录必须不存在或为空: $install_dir" >&2
  exit 1
fi
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
tar -C "$tmp" -xf "$bundle"
(cd "$tmp" && sha256sum -c SHA256SUMS)
docker load -i "$tmp/images.tar"
mkdir -p "$install_dir"
for file in docker-compose.yml .env.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh diagnostics.sh IMAGES SHA256SUMS; do
  cp "$tmp/$file" "$install_dir/$file"
done
cp -R "$tmp/local-agent" "$install_dir/local-agent"
cp -R "$tmp/docs" "$install_dir/docs"
cp -R "$tmp/bin" "$install_dir/bin"
mkdir -p "$install_dir/secrets" "$install_dir/backups"
chmod 0755 "$install_dir/preflight.sh" "$install_dir/render-config.sh" "$install_dir/backup.sh" "$install_dir/restore.sh" "$install_dir/diagnostics.sh"
chmod 0755 "$install_dir/local-agent/"*.sh
chmod 0755 "$install_dir/bin/licensectl" "$install_dir/bin/appforgectl"
echo "离线镜像、控制面部署文件、Local Agent安装包和企业文档已安装到 ${install_dir}；复制 .env.example 为 .env，放入证书后执行 ./preflight.sh。"
