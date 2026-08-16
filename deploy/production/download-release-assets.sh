#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 3 ]] || {
  echo "用法: $0 VERSION GITHUB_REPOSITORY OUTPUT_DIRECTORY" >&2
  exit 1
}

version=$1
repository=$2
output=$3
cosign=${APPFORGE_COSIGN_BINARY:?APPFORGE_COSIGN_BINARY required}
issuer=${APPFORGE_RELEASE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}

[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "版本必须是无 v 的语义化版本" >&2; exit 1; }
[[ $repository =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || { echo "GitHub 仓库名不合法" >&2; exit 1; }
[[ -f $cosign && ! -L $cosign && -x $cosign ]] || { echo "Cosign 必须是可执行普通文件" >&2; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "缺少 curl" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "缺少 jq" >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 || { echo "缺少 sha256sum" >&2; exit 1; }
command -v tar >/dev/null 2>&1 || { echo "缺少 tar" >&2; exit 1; }

if [[ -e $output ]]; then
  [[ -d $output && ! -L $output ]] || { echo "输出必须是普通目录" >&2; exit 1; }
  [[ -z $(find "$output" -mindepth 1 -maxdepth 1 -print -quit) ]] || { echo "输出目录必须为空" >&2; exit 1; }
fi

tag="v$version"
security_root="release-security-$tag"
tools_root="delivery-tools-$tag"
security_archive="appforge-release-security-$tag.tar.gz"
tools_archive="appforge-delivery-tools-$tag.tar.gz"
checksums=APPFORGE-RELEASE-ASSETS-SHA256SUMS
bundle=APPFORGE-RELEASE-ASSETS-SHA256SUMS.sigstore.json
base_url="https://github.com/$repository/releases/download/$tag"
identity="https://github.com/$repository/.github/workflows/release-security.yml@refs/tags/$tag"
registry="ghcr.io/${repository%%/*}/appforge"
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
downloads="$temporary/downloads"
extracted="$temporary/extracted"
mkdir -p "$downloads" "$extracted"

for file in "$security_archive" "$tools_archive" "$checksums" "$bundle"; do
  curl --fail --location --silent --show-error \
    --proto '=https' --proto-redir '=https' --tlsv1.2 \
    --output "$downloads/$file" "$base_url/$file"
  [[ -f $downloads/$file && ! -L $downloads/$file && -s $downloads/$file ]] || {
    echo "发布资产下载为空或不是普通文件: $file" >&2
    exit 1
  }
done

expected_names=$(printf '%s\n%s\n' "$security_archive" "$tools_archive" | LC_ALL=C sort)
actual_names=$(awk 'NF == 2 {name=$2; sub(/^\*/, "", name); print name}' "$downloads/$checksums" | LC_ALL=C sort)
[[ $actual_names == "$expected_names" ]] || { echo "发布资产 SHA 清单文件集合无效" >&2; exit 1; }
(
  cd "$downloads"
  sha256sum -c "$checksums" >/dev/null
)
"$cosign" verify-blob \
  --bundle "$downloads/$bundle" \
  --certificate-identity "$identity" \
  --certificate-oidc-issuer "$issuer" \
  "$downloads/$checksums" >/dev/null

validate_archive() {
  local archive=$1
  local expected_root=$2
  local entry
  while IFS= read -r entry; do
    [[ -n $entry ]] || continue
    case "$entry" in
      /*|../*|*/../*|*/..) echo "压缩包包含越界路径: $entry" >&2; return 1 ;;
    esac
    [[ $entry == "$expected_root" || $entry == "$expected_root/"* ]] || {
      echo "压缩包包含未登记根目录: $entry" >&2
      return 1
    }
  done < <(tar -tzf "$archive")
  tar -tvzf "$archive" | awk '$1 !~ /^[d-]/ {exit 1}' || {
    echo "压缩包只允许普通文件和目录" >&2
    return 1
  }
}

validate_archive "$downloads/$security_archive" "$security_root"
validate_archive "$downloads/$tools_archive" "$tools_root"
tar -xzf "$downloads/$security_archive" -C "$extracted"
tar -xzf "$downloads/$tools_archive" -C "$extracted"

APPFORGE_REQUIRE_COSIGN_VERIFY=1 \
APPFORGE_COSIGN_BINARY="$cosign" \
APPFORGE_RELEASE_CERTIFICATE_IDENTITY="$identity" \
APPFORGE_RELEASE_OIDC_ISSUER="$issuer" \
  "$script_dir/validate-release-evidence.sh" \
    "$extracted/$security_root" "$version" "$registry" >/dev/null

tools_directory="$extracted/$tools_root"
tools_expected=$(printf '%s\n' \
  SHA256SUMS SHA256SUMS.sigstore.json \
  appforgectl-linux-amd64 appforgectl-linux-arm64 \
  licensectl-linux-amd64 licensectl-linux-arm64 | LC_ALL=C sort)
tools_actual=$(find "$tools_directory" -mindepth 1 -maxdepth 1 -type f -print | sed 's#^.*/##' | LC_ALL=C sort)
[[ $tools_actual == "$tools_expected" ]] || { echo "交付工具文件集合无效" >&2; exit 1; }
"$cosign" verify-blob \
  --bundle "$tools_directory/SHA256SUMS.sigstore.json" \
  --certificate-identity "$identity" \
  --certificate-oidc-issuer "$issuer" \
  "$tools_directory/SHA256SUMS" >/dev/null
(
  cd "$tools_directory"
  sha256sum -c SHA256SUMS >/dev/null
)
for file in appforgectl-linux-amd64 appforgectl-linux-arm64 licensectl-linux-amd64 licensectl-linux-arm64; do
  [[ -x $tools_directory/$file ]] || { echo "交付工具不可执行: $file" >&2; exit 1; }
done

mkdir -p "$output/release-assets"
cp -R "$extracted/." "$output/"
cp "$downloads/$security_archive" "$downloads/$tools_archive" \
  "$downloads/$checksums" "$downloads/$bundle" "$output/release-assets/"
echo "公开发布资产下载与双层签名校验通过: $output"
