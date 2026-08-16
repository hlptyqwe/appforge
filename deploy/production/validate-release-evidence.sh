#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 3 ]] || { echo "用法: $0 EVIDENCE_DIRECTORY VERSION IMAGE_REGISTRY" >&2; exit 1; }
evidence=$1
version=$2
registry=${3%/}
components=(system core builder builder-worker api admin-ui agent-ui local-agent egress-proxy mysql etcd minio minio-mc etcd-init migrate mysql-binlog-tools)
third_party_components=(redis alpine)
third_party_repositories=(redis alpine)

[[ -d $evidence && ! -L $evidence ]] || { echo "证据目录不存在或为符号链接: $evidence" >&2; exit 1; }
[[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "版本必须是无 v 的语义化版本" >&2; exit 1; }
[[ $registry =~ ^[A-Za-z0-9._:/-]+$ ]] || { echo "镜像仓库不合法" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "缺少 jq" >&2; exit 1; }

for file in "$evidence"/*; do
  [[ -f $file && ! -L $file ]] || { echo "证据目录只允许普通文件: $file" >&2; exit 1; }
done
[[ -f $evidence/RELEASE-MANIFEST.json && -f $evidence/SHA256SUMS ]] || { echo "缺少发布清单或 SHA 清单" >&2; exit 1; }
[[ -f $evidence/SHA256SUMS.sigstore.json ]] || { echo "缺少发布证据 Sigstore 签名包" >&2; exit 1; }
jq -e 'type == "object"' "$evidence/SHA256SUMS.sigstore.json" >/dev/null || { echo "Sigstore 签名包不是 JSON 对象" >&2; exit 1; }

expected_names=$(
  {
    printf '%s\n' RELEASE-MANIFEST.json
    for component in "${components[@]}"; do
      printf '%s\n' "$component.cosign.json" "$component.image.txt" "$component.licenses.json" "$component.spdx.json" "$component.trivy.sarif"
    done
    for component in "${third_party_components[@]}"; do
      printf '%s\n' "third-party-$component.image.txt" "third-party-$component.licenses.json" \
        "third-party-$component.spdx.json" "third-party-$component.trivy.sarif"
    done
  } | LC_ALL=C sort
)
actual_names=$(awk 'NF == 2 {print $2}' "$evidence/SHA256SUMS" | LC_ALL=C sort)
[[ $actual_names == "$expected_names" ]] || { echo "SHA256SUMS 文件集合与固定交付契约不一致" >&2; exit 1; }
expected_directory_names=$(printf '%s\n%s\nSHA256SUMS\nSHA256SUMS.sigstore.json\n' "$expected_names" "" | sed '/^$/d' | LC_ALL=C sort)
actual_directory_names=$(find "$evidence" -mindepth 1 -maxdepth 1 -type f -print | sed 's#^.*/##' | LC_ALL=C sort)
[[ $actual_directory_names == "$expected_directory_names" ]] || { echo "发布证据目录包含缺失或未登记文件" >&2; exit 1; }
(cd "$evidence" && sha256sum -c SHA256SUMS >/dev/null) || { echo "发布证据 SHA 校验失败" >&2; exit 1; }

jq -e --arg version "$version" --arg registry "$registry" \
  --argjson count "${#components[@]}" --argjson thirdPartyCount "${#third_party_components[@]}" '
  .schemaVersion == 1 and .version == $version and .registry == $registry and
  (.gitRef == ("refs/tags/v" + $version)) and
  (.gitCommit | test("^[0-9a-f]{40}$")) and
  (.components | type == "array" and length == $count) and
  (.thirdPartyImages | type == "array" and length == $thirdPartyCount) and
  .sourceDependencyGate == {
    status: "passed",
    scanner: "trivy-fs",
    scope: "repository-manifests-and-lockfiles",
    detailsExported: false
  }
' "$evidence/RELEASE-MANIFEST.json" >/dev/null || { echo "发布清单元数据无效" >&2; exit 1; }

for component in "${components[@]}"; do
  image_ref=$(tr -d '\r\n' <"$evidence/$component.image.txt")
  expected_prefix="$registry/$component@sha256:"
  [[ $image_ref == "$expected_prefix"* ]] || { echo "镜像引用无效: $image_ref" >&2; exit 1; }
  digest=${image_ref#"$expected_prefix"}
  [[ $digest =~ ^[0-9a-f]{64}$ ]] || { echo "镜像 digest 无效: $image_ref" >&2; exit 1; }
  jq -e --arg component "$component" --arg image "$image_ref" '
    [.components[] | select(.component == $component and .image == $image and .digest == ($image | split("@")[1]))] | length == 1
  ' "$evidence/RELEASE-MANIFEST.json" >/dev/null || { echo "发布清单与组件镜像不一致: $component" >&2; exit 1; }
  jq -e '(.spdxVersion | startswith("SPDX-")) and (.packages | length > 0)' "$evidence/$component.spdx.json" >/dev/null
  jq -e --arg component "$component" '.schemaVersion == 1 and .component == $component and (.packages | length > 0)' "$evidence/$component.licenses.json" >/dev/null
  jq -e '.version == "2.1.0" and (.runs | length > 0)' "$evidence/$component.trivy.sarif" >/dev/null
  jq -e 'type == "array" and length > 0' "$evidence/$component.cosign.json" >/dev/null
done

for index in "${!third_party_components[@]}"; do
  component=${third_party_components[$index]}
  repository_name=${third_party_repositories[$index]}
  evidence_name="third-party-$component"
  image_ref=$(tr -d '\r\n' <"$evidence/$evidence_name.image.txt")
  expected_prefix="$repository_name@sha256:"
  [[ $image_ref == "$expected_prefix"* ]] || { echo "第三方镜像引用无效: $image_ref" >&2; exit 1; }
  digest=${image_ref#"$expected_prefix"}
  [[ $digest =~ ^[0-9a-f]{64}$ ]] || { echo "第三方镜像 digest 无效: $image_ref" >&2; exit 1; }
  jq -e --arg component "$component" --arg image "$image_ref" '
    [.thirdPartyImages[] | select(
      .component == $component and .image == $image and
      .digest == ($image | split("@")[1]) and .trust == "aggregate-manifest-signature"
    )] | length == 1
  ' "$evidence/RELEASE-MANIFEST.json" >/dev/null || { echo "发布清单与第三方镜像不一致: $component" >&2; exit 1; }
  jq -e '(.spdxVersion | startswith("SPDX-")) and (.packages | length > 0)' "$evidence/$evidence_name.spdx.json" >/dev/null
  jq -e --arg component "$evidence_name" '.schemaVersion == 1 and .component == $component and (.packages | length > 0)' "$evidence/$evidence_name.licenses.json" >/dev/null
  jq -e '.version == "2.1.0" and (.runs | length > 0)' "$evidence/$evidence_name.trivy.sarif" >/dev/null
done

if [[ ${APPFORGE_REQUIRE_COSIGN_VERIFY:-0} == 1 ]]; then
  cosign=${APPFORGE_COSIGN_BINARY:?APPFORGE_COSIGN_BINARY required}
  identity=${APPFORGE_RELEASE_CERTIFICATE_IDENTITY:?APPFORGE_RELEASE_CERTIFICATE_IDENTITY required}
  issuer=${APPFORGE_RELEASE_OIDC_ISSUER:-https://token.actions.githubusercontent.com}
  [[ -f $cosign && ! -L $cosign && -x $cosign ]] || { echo "Cosign 必须是可执行普通文件: $cosign" >&2; exit 1; }
  "$cosign" verify-blob \
    --bundle "$evidence/SHA256SUMS.sigstore.json" \
    --certificate-identity "$identity" \
    --certificate-oidc-issuer "$issuer" \
    "$evidence/SHA256SUMS" >/dev/null
fi

echo "发布安全证据校验通过: $version"
