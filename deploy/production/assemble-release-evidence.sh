#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 6 ]] || {
  echo "用法: $0 INPUT_DIRECTORY OUTPUT_DIRECTORY VERSION GITHUB_REPOSITORY GITHUB_REF GIT_COMMIT" >&2
  exit 1
}
input=$1
output=$2
version=$3
repository=$4
git_ref=$5
git_commit=$6
source_dependency_gate=${APPFORGE_SOURCE_DEPENDENCY_GATE:?APPFORGE_SOURCE_DEPENDENCY_GATE required}
components=(system core builder builder-worker api admin-ui agent-ui local-agent egress-proxy etcd-init migrate mysql-binlog-tools)
third_party_components=(mysql redis etcd minio minio-mc alpine)
third_party_repositories=(mysql redis quay.io/coreos/etcd minio/minio minio/mc alpine)

[[ -d $input && ! -L $input ]] || { echo "输入目录不存在或为符号链接: $input" >&2; exit 1; }
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "版本必须是无 v 的语义化版本: $version" >&2; exit 1; }
[[ $repository =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || { echo "GitHub 仓库名不合法: $repository" >&2; exit 1; }
[[ $git_ref == "refs/tags/v$version" ]] || { echo "Git ref 与版本不匹配: $git_ref" >&2; exit 1; }
[[ $git_commit =~ ^[0-9a-fA-F]{40}$ ]] || { echo "Git commit 必须是 40 位 SHA: $git_commit" >&2; exit 1; }
[[ $source_dependency_gate == success ]] || { echo "源码依赖漏洞门禁未通过: $source_dependency_gate" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "缺少 jq" >&2; exit 1; }

if [[ -e $output ]] && [[ -n $(find "$output" -mindepth 1 -maxdepth 1 -print -quit) ]]; then
  echo "输出目录必须不存在或为空: $output" >&2
  exit 1
fi
mkdir -p "$output"
chmod 0750 "$output"

owner=${repository%%/*}
registry="ghcr.io/$owner/appforge"
manifest_components='[]'
for component in "${components[@]}"; do
  for suffix in spdx.json licenses.json trivy.sarif cosign.json image.txt; do
    file="$input/$component.$suffix"
    [[ -f $file && ! -L $file ]] || { echo "缺少普通证据文件: $file" >&2; exit 1; }
  done

  jq -e '(.spdxVersion | type == "string" and startswith("SPDX-")) and (.packages | type == "array" and length > 0)' \
    "$input/$component.spdx.json" >/dev/null || { echo "SPDX 无效: $component" >&2; exit 1; }
  jq -e --arg component "$component" \
    '.schemaVersion == 1 and .component == $component and (.packages | type == "array" and length > 0)' \
    "$input/$component.licenses.json" >/dev/null || { echo "许可证清单无效: $component" >&2; exit 1; }
  jq -e '.version == "2.1.0" and (.runs | type == "array" and length > 0)' \
    "$input/$component.trivy.sarif" >/dev/null || { echo "SARIF 无效: $component" >&2; exit 1; }
  jq -e 'type == "array" and length > 0' \
    "$input/$component.cosign.json" >/dev/null || { echo "Cosign 回验记录无效: $component" >&2; exit 1; }

  image_ref=$(tr -d '\r\n' <"$input/$component.image.txt")
  expected_prefix="$registry/$component@sha256:"
  [[ $image_ref == "$expected_prefix"* ]] || { echo "镜像引用不属于发布组件: $image_ref" >&2; exit 1; }
  digest=${image_ref##*@sha256:}
  [[ $digest =~ ^[0-9a-f]{64}$ ]] || { echo "镜像 digest 无效: $image_ref" >&2; exit 1; }

  cp "$input/$component.spdx.json" "$input/$component.licenses.json" \
    "$input/$component.trivy.sarif" "$input/$component.cosign.json" \
    "$input/$component.image.txt" "$output/"
  manifest_components=$(jq -c \
    --arg component "$component" --arg image "$image_ref" --arg digest "sha256:$digest" \
    '. + [{component: $component, image: $image, digest: $digest}]' <<<"$manifest_components")
done

manifest_third_party='[]'
for index in "${!third_party_components[@]}"; do
  component=${third_party_components[$index]}
  repository_name=${third_party_repositories[$index]}
  evidence_name="third-party-$component"
  for suffix in spdx.json licenses.json trivy.sarif image.txt; do
    file="$input/$evidence_name.$suffix"
    [[ -f $file && ! -L $file ]] || { echo "缺少第三方普通证据文件: $file" >&2; exit 1; }
  done
  jq -e '(.spdxVersion | type == "string" and startswith("SPDX-")) and (.packages | type == "array" and length > 0)' \
    "$input/$evidence_name.spdx.json" >/dev/null || { echo "第三方 SPDX 无效: $component" >&2; exit 1; }
  jq -e --arg component "$evidence_name" \
    '.schemaVersion == 1 and .component == $component and (.packages | type == "array" and length > 0)' \
    "$input/$evidence_name.licenses.json" >/dev/null || { echo "第三方许可证清单无效: $component" >&2; exit 1; }
  jq -e '.version == "2.1.0" and (.runs | type == "array" and length > 0)' \
    "$input/$evidence_name.trivy.sarif" >/dev/null || { echo "第三方 SARIF 无效: $component" >&2; exit 1; }

  image_ref=$(tr -d '\r\n' <"$input/$evidence_name.image.txt")
  expected_prefix="$repository_name@sha256:"
  [[ $image_ref == "$expected_prefix"* ]] || { echo "第三方镜像引用无效: $image_ref" >&2; exit 1; }
  digest=${image_ref##*@sha256:}
  [[ $digest =~ ^[0-9a-f]{64}$ ]] || { echo "第三方镜像 digest 无效: $image_ref" >&2; exit 1; }

  cp "$input/$evidence_name.spdx.json" "$input/$evidence_name.licenses.json" \
    "$input/$evidence_name.trivy.sarif" "$input/$evidence_name.image.txt" "$output/"
  manifest_third_party=$(jq -c \
    --arg component "$component" --arg image "$image_ref" --arg digest "sha256:$digest" \
    '. + [{component: $component, image: $image, digest: $digest, trust: "aggregate-manifest-signature"}]' \
    <<<"$manifest_third_party")
done

jq -n \
  --arg version "$version" \
  --arg repository "$repository" \
  --arg gitRef "$git_ref" \
  --arg gitCommit "${git_commit,,}" \
  --arg registry "$registry" \
  --argjson components "$manifest_components" \
  --argjson thirdPartyImages "$manifest_third_party" \
  '{
    schemaVersion: 1,
    version: $version,
    repository: $repository,
    gitRef: $gitRef,
    gitCommit: $gitCommit,
    registry: $registry,
    components: $components,
    thirdPartyImages: $thirdPartyImages,
    sourceDependencyGate: {
      status: "passed",
      scanner: "trivy-fs",
      scope: "repository-manifests-and-lockfiles",
      detailsExported: false
    }
  }' >"$output/RELEASE-MANIFEST.json"

(
  cd "$output"
  find . -maxdepth 1 -type f ! -name SHA256SUMS -print | sed 's#^./##' | LC_ALL=C sort |
    while IFS= read -r file; do sha256sum "$file"; done >SHA256SUMS
)
chmod 0640 "$output"/*
echo "发布安全证据已汇总: $output"
