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
grep -q 'mysql-binlog-tools' "$repo_root/deploy/production/offline-bundle.sh" || {
  echo "验收失败: 离线包未包含MySQL PITR工具镜像" >&2
  exit 1
}
mkdir -p "$package_root/docs" "$package_root/bin"

for file in docker-compose.yml .env.example egress-allowlist.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh archive-binlogs.sh pitr-restore.sh configure-object-replication.sh diagnostics.sh; do
  cp "$repo_root/deploy/production/$file" "$package_root/$file"
done
cp -R "$repo_root/deploy/local-agent" "$package_root/local-agent"
for file in delivery-guide.md local-agent-artifact-protocol.md customer-storage-contract.md air-gapped-artifact-contract.md remote-apk-signing-contract.md restricted-egress-proxy.md secret-providers.md version-compatibility.md; do
  cp "$repo_root/docs/enterprise/$file" "$package_root/docs/$file"
done
cp "$repo_root/deploy/production/preflight.sh" "$package_root/bin/licensectl"
cp "$repo_root/deploy/production/preflight.sh" "$package_root/bin/appforgectl"
cp "$repo_root/deploy/production/validate-release-evidence.sh" "$package_root/bin/validate-release-evidence"
chmod 0755 "$package_root/bin/licensectl" "$package_root/bin/appforgectl" "$package_root/bin/validate-release-evidence"

fixture_security_input="$temporary/security-input"
mkdir -p "$fixture_security_input"
components=(system core builder builder-worker api admin-ui agent-ui local-agent egress-proxy etcd-init migrate mysql-binlog-tools)
for component in "${components[@]}"; do
  digest=$(printf '%s' "$component" | sha256sum | awk '{print $1}')
  jq -n --arg component "$component" '{
    spdxVersion: "SPDX-2.3",
    documentNamespace: ("urn:appforge:offline-layout:" + $component),
    packages: [{name: $component, versionInfo: "1.2.3", licenseDeclared: "Apache-2.0", licenseConcluded: "Apache-2.0"}]
  }' >"$fixture_security_input/$component.spdx.json"
  "$repo_root/deploy/production/generate-license-inventory.sh" \
    "$component" "$fixture_security_input/$component.spdx.json" "$fixture_security_input/$component.licenses.json"
  jq -n '{version: "2.1.0", runs: [{tool: {driver: {name: "Trivy"}}, results: []}]}' \
    >"$fixture_security_input/$component.trivy.sarif"
  jq -n --arg component "$component" '[{critical: {identity: $component}}]' \
    >"$fixture_security_input/$component.cosign.json"
  printf 'ghcr.io/appforge-acceptance/appforge/%s@sha256:%s\n' "$component" "$digest" \
    >"$fixture_security_input/$component.image.txt"
done
third_party_components=(mysql redis etcd minio minio-mc alpine)
third_party_repositories=(mysql redis quay.io/coreos/etcd minio/minio minio/mc alpine)
for index in "${!third_party_components[@]}"; do
  component=${third_party_components[$index]}
  repository_name=${third_party_repositories[$index]}
  evidence_name="third-party-$component"
  digest=$(printf '%s' "$evidence_name" | sha256sum | awk '{print $1}')
  jq -n --arg component "$evidence_name" '{
    spdxVersion: "SPDX-2.3",
    documentNamespace: ("urn:appforge:offline-layout:" + $component),
    packages: [{name: $component, versionInfo: "1.2.3", licenseDeclared: "Apache-2.0", licenseConcluded: "Apache-2.0"}]
  }' >"$fixture_security_input/$evidence_name.spdx.json"
  "$repo_root/deploy/production/generate-license-inventory.sh" \
    "$evidence_name" "$fixture_security_input/$evidence_name.spdx.json" "$fixture_security_input/$evidence_name.licenses.json"
  jq -n '{version: "2.1.0", runs: [{tool: {driver: {name: "Trivy"}}, results: []}]}' \
    >"$fixture_security_input/$evidence_name.trivy.sarif"
  printf '%s@sha256:%s\n' "$repository_name" "$digest" >"$fixture_security_input/$evidence_name.image.txt"
done
APPFORGE_SOURCE_DEPENDENCY_GATE=success \
  "$repo_root/deploy/production/assemble-release-evidence.sh" \
  "$fixture_security_input" "$package_root/security" 1.2.3 appforge-acceptance/appforge \
  refs/tags/v1.2.3 0123456789abcdef0123456789abcdef01234567 >/dev/null
printf '{}\n' >"$package_root/security/SHA256SUMS.sigstore.json"
"$repo_root/deploy/production/validate-release-evidence.sh" \
  "$package_root/security" 1.2.3 ghcr.io/appforge-acceptance/appforge >/dev/null

fake_docker="$temporary/fake-docker"
fake_cosign="$temporary/fake-cosign"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'case ${1:-} in' \
  '  pull|tag) exit 0 ;;' \
  '  save)' \
  '    shift' \
  '    output=' \
  '    while [[ $# -gt 0 ]]; do' \
  '      if [[ $1 == -o ]]; then output=$2; shift 2; else shift; fi' \
  '    done' \
  '    [[ -n $output ]]' \
  '    printf "synthetic-oci-layout\n" >"$output"' \
  '    ;;' \
  '  *) echo "unexpected docker command" >&2; exit 1 ;;' \
  'esac' >"$fake_docker"
printf '%s\n' '#!/usr/bin/env bash' '[[ ${1:-} == verify-blob ]]' >"$fake_cosign"
chmod 0755 "$fake_docker" "$fake_cosign"
production_bundle="$temporary/production-offline.tar"
APPFORGE_IMAGE_REGISTRY=ghcr.io/appforge-acceptance/appforge \
APPFORGE_LICENSECTL_BINARY="$package_root/bin/licensectl" \
APPFORGE_APPFORGECTL_BINARY="$package_root/bin/appforgectl" \
APPFORGE_COSIGN_BINARY="$fake_cosign" \
APPFORGE_DOCKER_BINARY="$fake_docker" \
APPFORGE_RELEASE_CERTIFICATE_IDENTITY=https://github.com/appforge-acceptance/appforge/.github/workflows/release-security.yml@refs/tags/v1.2.3 \
  "$repo_root/deploy/production/offline-bundle.sh" \
    1.2.3 "$package_root/security" "$production_bundle" >/dev/null
tar -tf "$production_bundle" | rg -q '^\./security/RELEASE-MANIFEST.json$'
tar -tf "$production_bundle" | rg -q '^\./bin/validate-release-evidence$'
[[ $(tar -xOf "$production_bundle" ./IMAGES | wc -l | tr -d ' ') == 18 ]] || {
  echo "验收失败: 生产离线包镜像清单不是18个固定镜像" >&2
  exit 1
}

printf '%s\n' "$fixture_image" >"$package_root/IMAGES"
docker image inspect "$fixture_image" >/dev/null
docker save "$fixture_image" -o "$package_root/images.tar"
(cd "$package_root" && {
  sha256sum images.tar docker-compose.yml .env.example egress-allowlist.example nginx.conf preflight.sh render-config.sh backup.sh restore.sh archive-binlogs.sh pitr-restore.sh configure-object-replication.sh diagnostics.sh IMAGES
  find local-agent docs bin security -type f | LC_ALL=C sort | while IFS= read -r file; do sha256sum "$file"; done
} >SHA256SUMS)
tar -C "$package_root" -cf "$temporary/offline.tar" .

"$repo_root/deploy/production/offline-install.sh" "$temporary/offline.tar" "$install_root" >/dev/null
for required in diagnostics.sh archive-binlogs.sh pitr-restore.sh configure-object-replication.sh egress-allowlist.example local-agent/docker-compose.yml local-agent/register.sh local-agent/secret-import.sh \
  local-agent/customer-storage-secret-import.sh local-agent/customer-storage-import.sh \
  local-agent/upgrade.sh docs/delivery-guide.md docs/air-gapped-artifact-contract.md docs/remote-apk-signing-contract.md docs/restricted-egress-proxy.md docs/version-compatibility.md; do
  [[ -f "$install_root/$required" ]] || { echo "验收失败: 离线安装缺少 $required" >&2; exit 1; }
done
for required in security/RELEASE-MANIFEST.json security/SHA256SUMS security/SHA256SUMS.sigstore.json \
  security/api.spdx.json security/api.licenses.json security/api.trivy.sarif security/api.cosign.json \
  security/third-party-mysql.spdx.json security/third-party-mysql.licenses.json security/third-party-mysql.trivy.sarif \
  bin/validate-release-evidence; do
  [[ -f "$install_root/$required" ]] || { echo "验收失败: 离线安装缺少发布安全证据 $required" >&2; exit 1; }
done
[[ -x "$install_root/diagnostics.sh" && -x "$install_root/local-agent/register.sh" ]] || {
  echo "验收失败: 离线安装脚本没有可执行权限" >&2; exit 1;
}
[[ -x "$install_root/bin/licensectl" && -x "$install_root/bin/appforgectl" ]] || {
  echo "验收失败: 离线安装缺少可执行交付CLI" >&2; exit 1;
}
[[ -x "$install_root/bin/validate-release-evidence" ]] || {
  echo "验收失败: 离线安装缺少可执行发布证据校验器" >&2; exit 1;
}
echo "通过: 离线包SHA校验、OCI导入、签名发布安全证据、控制面文件、CLI、Local Agent安装包和企业文档落盘"
