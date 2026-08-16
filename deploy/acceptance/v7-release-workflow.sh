#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
workflow="$repo_root/.github/workflows/release-security.yml"
report_file=${APPFORGE_RELEASE_EVIDENCE_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-release-evidence-contract-20260816.json}
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT

ruby -e 'require "yaml"; YAML.load_file(ARGV.fetch(0))' "$workflow" 2>/dev/null
grep -q 'docker/metadata-action@v5' "$workflow"
grep -q 'type=semver,pattern={{version}}' "$workflow"
grep -q 'APPFORGE_SCHEMA_TARGET=20260815_113_v7_air_gapped' "$workflow"
grep -q 'steps.build.outputs.digest' "$workflow"
grep -q 'format: sarif' "$workflow"
grep -q 'cosign verify ' "$workflow"
grep -q 'cosign verify-blob' "$workflow"
grep -q 'generate-license-inventory.sh' "$workflow"
grep -q 'assemble-release-evidence.sh' "$workflow"
grep -q 'validate-release-evidence.sh' "$workflow"
grep -q 'release-security-${{ github.ref_name }}' "$workflow"
grep -q 'third-party-images:' "$workflow"
grep -q 'Third-party vulnerability gate' "$workflow"
grep -q 'release-evidence-third-party-' "$workflow"
grep -q 'source-dependency-gate:' "$workflow"
grep -q 'Source dependency vulnerability gate without report export' "$workflow"
grep -q 'aquasecurity/trivy-action@v0.36.0' "$workflow"
if rg -q 'source\.(spdx|licenses|trivy)' "$workflow"; then
  echo "验收失败: 源码依赖明细不得导出到工作流 Artifact 或 SARIF" >&2
  exit 1
fi
grep -q 'licensectl-linux-' "$workflow"
grep -q 'appforgectl-linux-' "$workflow"
grep -q 'component: egress-proxy' "$workflow"
grep -q 'MODULE_DIR=common/cmd/egress-proxy' "$workflow"
for component in mysql etcd minio minio-mc; do
  grep -q "component: $component" "$workflow"
  grep -q "dockerfile: deploy/docker/hardened-$component.Dockerfile" "$workflow"
done
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

fixture_input="$temporary/release-input"
fixture_output="$temporary/release-security"
mkdir -p "$fixture_input"
components=(system core builder builder-worker api admin-ui agent-ui local-agent egress-proxy mysql etcd minio minio-mc etcd-init migrate mysql-binlog-tools)
for component in "${components[@]}"; do
  digest=$(printf '%s' "$component" | sha256sum | awk '{print $1}')
  jq -n --arg component "$component" '{
    spdxVersion: "SPDX-2.3",
    documentNamespace: ("urn:appforge:acceptance:" + $component),
    packages: [{name: $component, versionInfo: "1.2.3", licenseDeclared: "Apache-2.0", licenseConcluded: "Apache-2.0"}]
  }' >"$fixture_input/$component.spdx.json"
  "$repo_root/deploy/production/generate-license-inventory.sh" \
    "$component" "$fixture_input/$component.spdx.json" "$fixture_input/$component.licenses.json"
  jq -n '{version: "2.1.0", runs: [{tool: {driver: {name: "Trivy"}}, results: []}]}' \
    >"$fixture_input/$component.trivy.sarif"
  jq -n --arg component "$component" '[{critical: {identity: $component}, optional: null}]' \
    >"$fixture_input/$component.cosign.json"
  printf 'ghcr.io/appforge-acceptance/appforge/%s@sha256:%s\n' "$component" "$digest" \
    >"$fixture_input/$component.image.txt"
done
third_party_components=(redis alpine)
third_party_repositories=(redis alpine)
for index in "${!third_party_components[@]}"; do
  component=${third_party_components[$index]}
  repository_name=${third_party_repositories[$index]}
  evidence_name="third-party-$component"
  digest=$(printf '%s' "$evidence_name" | sha256sum | awk '{print $1}')
  jq -n --arg component "$evidence_name" '{
    spdxVersion: "SPDX-2.3",
    documentNamespace: ("urn:appforge:acceptance:" + $component),
    packages: [{name: $component, versionInfo: "1.2.3", licenseDeclared: "Apache-2.0", licenseConcluded: "Apache-2.0"}]
  }' >"$fixture_input/$evidence_name.spdx.json"
  "$repo_root/deploy/production/generate-license-inventory.sh" \
    "$evidence_name" "$fixture_input/$evidence_name.spdx.json" "$fixture_input/$evidence_name.licenses.json"
  jq -n '{version: "2.1.0", runs: [{tool: {driver: {name: "Trivy"}}, results: []}]}' \
    >"$fixture_input/$evidence_name.trivy.sarif"
  printf '%s@sha256:%s\n' "$repository_name" "$digest" >"$fixture_input/$evidence_name.image.txt"
done
if APPFORGE_SOURCE_DEPENDENCY_GATE=failure \
  "$repo_root/deploy/production/assemble-release-evidence.sh" \
    "$fixture_input" "$temporary/rejected-release-security" 1.2.3 appforge-acceptance/appforge \
    refs/tags/v1.2.3 0123456789abcdef0123456789abcdef01234567 >/dev/null 2>&1; then
  echo "验收失败: 源码依赖漏洞门禁失败后仍能汇总发布证据" >&2
  exit 1
fi
APPFORGE_SOURCE_DEPENDENCY_GATE=success \
  "$repo_root/deploy/production/assemble-release-evidence.sh" \
  "$fixture_input" "$fixture_output" 1.2.3 appforge-acceptance/appforge \
  refs/tags/v1.2.3 0123456789abcdef0123456789abcdef01234567
printf '{}\n' >"$fixture_output/SHA256SUMS.sigstore.json"
"$repo_root/deploy/production/validate-release-evidence.sh" \
  "$fixture_output" 1.2.3 ghcr.io/appforge-acceptance/appforge
if "$repo_root/deploy/production/validate-release-evidence.sh" \
  "$fixture_output" 1.2.4 ghcr.io/appforge-acceptance/appforge >/dev/null 2>&1; then
  echo "验收失败: 跨版本发布证据仍被接受" >&2
  exit 1
fi
if "$repo_root/deploy/production/validate-release-evidence.sh" \
  "$fixture_output" 1.2.3 ghcr.io/other-owner/appforge >/dev/null 2>&1; then
  echo "验收失败: 跨仓库发布证据仍被接受" >&2
  exit 1
fi
printf 'not-registered\n' >"$fixture_output/extra.txt"
if "$repo_root/deploy/production/validate-release-evidence.sh" \
  "$fixture_output" 1.2.3 ghcr.io/appforge-acceptance/appforge >/dev/null 2>&1; then
  echo "验收失败: 未登记发布证据文件仍被接受" >&2
  exit 1
fi
rm "$fixture_output/extra.txt"
printf '\n' >>"$fixture_output/api.spdx.json"
if "$repo_root/deploy/production/validate-release-evidence.sh" \
  "$fixture_output" 1.2.3 ghcr.io/appforge-acceptance/appforge >/dev/null 2>&1; then
  echo "验收失败: 篡改后的发布证据仍被接受" >&2
  exit 1
fi

umask 077
mkdir -p "$(dirname "$report_file")"
jq -n \
  --arg generatedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson components "$(printf '%s\n' "${components[@]}" | jq -R . | jq -s .)" \
  '{
    schemaVersion: 1,
    kind: "appforge-v7-release-evidence-contract-acceptance",
    generatedAt: $generatedAt,
    result: "passed",
    mode: "synthetic-local-contract",
    releaseVersion: "1.2.3",
    components: $components,
    checks: {
      tagOnlyTrigger: true,
      immutableDigestReferences: true,
      spdxSbom: true,
      normalizedLicenseInventory: true,
      trivySarif: true,
      perImageCosignVerificationRecord: true,
      aggregateManifest: true,
      aggregateSha256: true,
      sourceDependencyGateRequired: true,
      sourceDependencyDetailsExported: false,
      tamperRejected: true,
      dualArchitectureCliBuild: true
    },
    thirdPartyImages: ["redis", "alpine"],
    boundary: "本地验收验证发布证据结构、聚合、防篡改和源码依赖门禁结果绑定；源码门禁状态与 Sigstore 包均为合成输入，不代表真实 tag、Trivy 漏洞数据库、OIDC 或镜像签名运行证据。"
  }' >"$report_file"
chmod 0600 "$report_file"

echo "通过: tag限定发布、源码依赖无明细导出门禁、16个自建签名与2个第三方镜像digest扫描、许可证清单、聚合签名契约、防篡改和双架构CLI构建"
echo "证据: $report_file"
