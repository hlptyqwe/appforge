#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
validator="$repo_root/deploy/production/validate-customer-site-evidence.sh"
assembler="$repo_root/deploy/production/assemble-customer-site-evidence.sh"
initializer="$repo_root/deploy/production/init-customer-site-evidence.sh"
report_file=${APPFORGE_CUSTOMER_SITE_EVIDENCE_CONTRACT_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-customer-site-evidence-contract-20260817.json}
[[ $report_file == /* ]] || { echo "验收失败: 证据路径必须是绝对路径" >&2; exit 1; }
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
gate_source="$temporary/gates"
bundle="$temporary/evidence"
mkdir -m 700 "$gate_source"
umask 077

for tool in jq sha256sum openssl; do
  command -v "$tool" >/dev/null 2>&1 || { echo "验收失败: 缺少 $tool" >&2; exit 1; }
done
bash -n "$validator" "$assembler" "$initializer"
grep -Fq 'init-customer-site-evidence.sh' "$repo_root/deploy/production/offline-bundle.sh"
grep -Fq 'assemble-customer-site-evidence.sh' "$repo_root/deploy/production/offline-bundle.sh"
grep -Fq 'validate-customer-site-evidence.sh' "$repo_root/deploy/production/offline-bundle.sh"
grep -Fq 'docs/customer-site-acceptance.md' "$repo_root/deploy/acceptance/v7-offline-layout.sh"
grep -Fq 'bin/init-customer-site-evidence' "$repo_root/deploy/acceptance/v7-offline-layout.sh"
grep -Fq 'bin/validate-customer-site-evidence' "$repo_root/deploy/acceptance/v7-offline-layout.sh"

openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$temporary/customer.key" >/dev/null 2>&1
openssl pkey -in "$temporary/customer.key" -pubout -out "$temporary/customer.pub" >/dev/null 2>&1
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$temporary/appforge.key" >/dev/null 2>&1
openssl pkey -in "$temporary/appforge.key" -pubout -out "$temporary/appforge.pub" >/dev/null 2>&1

site_fingerprint=$(printf '%s' synthetic-customer-site | sha256sum | awk '{print $1}')
git_commit=0123456789abcdef0123456789abcdef01234567
release_version=1.2.7
schema_version=20260815_113_v7_air_gapped
started_at=2026-08-17T00:00:00Z
finished_at=2026-08-18T00:00:01Z

template_directory="$temporary/templates"
"$initializer" "$template_directory" "$release_version" "$git_commit" "$site_fingerprint" customer-test >/dev/null
[[ -f $template_directory/metadata.json && -d $template_directory/gates ]] || {
  echo "验收失败: pending 模板缺少 metadata 或 gates 目录" >&2
  exit 1
}
[[ $(find "$template_directory/gates" -mindepth 1 -maxdepth 1 -type f | wc -l | tr -d ' ') == 8 ]] || {
  echo "验收失败: pending 模板不是固定 8 门禁" >&2
  exit 1
}
for template in "$template_directory/metadata.json" "$template_directory/gates"/*.json; do
  if [[ ${template##*/} == metadata.json ]]; then
    jq -e '.environmentKind=="customer-test" and .releaseVersion=="1.2.7" and .agentProtocol==3' "$template" >/dev/null
  else
    jq -e '.result=="pending" and .verified==[] and .openFindings==["pending-customer-execution"]' "$template" >/dev/null
  fi
  if stat -c '%a' "$template" >/dev/null 2>&1; then
    [[ $(stat -c '%a' "$template") == 600 ]] || { echo "验收失败: pending 模板权限不是0600" >&2; exit 1; }
  else
    [[ $(stat -f '%Lp' "$template") == 600 ]] || { echo "验收失败: pending 模板权限不是0600" >&2; exit 1; }
  fi
done
if "$assembler" "$template_directory/gates" "$temporary/pending-assembly" "$template_directory/metadata.json" \
  "$temporary/customer.pub" "$temporary/appforge.pub" >/dev/null 2>&1; then
  echo "验收失败: 未执行的 pending 模板被汇总" >&2
  exit 1
fi
[[ ! -e $temporary/pending-assembly ]] || { echo "验收失败: pending 汇总失败后遗留目录" >&2; exit 1; }

write_gate() {
  local gate_id=$1 verified=$2 metrics=$3 metrics_json
  metrics_json=$(jq -nc "$metrics")
  jq -n \
    --arg gateId "$gate_id" \
    --arg siteFingerprint "$site_fingerprint" \
    --arg releaseVersion "$release_version" \
    --arg targetSchemaVersion "$schema_version" \
    --arg gitCommit "$git_commit" \
    --arg startedAt "$started_at" \
    --arg finishedAt "$finished_at" \
    --argjson verified "$verified" \
    --argjson metrics "$metrics_json" '
    {
      schemaVersion: 1,
      evidenceType: "v7-customer-site-gate",
      gateId: $gateId,
      result: "passed",
      runId: ("acceptance-" + $gateId),
      environmentKind: "customer-test",
      siteFingerprint: $siteFingerprint,
      releaseVersion: $releaseVersion,
      targetSchemaVersion: $targetSchemaVersion,
      agentProtocol: 3,
      gitCommit: $gitCommit,
      startedAt: $startedAt,
      finishedAt: $finishedAt,
      verified: $verified,
      metrics: $metrics,
      openFindings: [],
      dataPolicy: {
        credentialsIncluded: false,
        rawCustomerDataIncluded: false,
        executionApprovalRef: "SYNTHETIC-CONTRACT-ONLY"
      }
    }
  ' >"$gate_source/$gate_id.json"
}

write_gate customer-object-storage \
  '["aws-s3","aliyun-oss","least-privilege-prefix","workload-identity-or-short-lived-credential","put-stat-full-get-sha256-delete-not-found","existing-objects-unchanged","zero-synthetic-residue"]' \
  '{providerResults:[
    {provider:"aws-s3",passed:true,leastPrivilegePrefix:true,fullReadbackSha256:true,deleteNotFoundConfirmed:true,existingObjectsUnchanged:true,syntheticResidueCount:0},
    {provider:"aliyun-oss",passed:true,leastPrivilegePrefix:true,fullReadbackSha256:true,deleteNotFoundConfirmed:true,existingObjectsUnchanged:true,syntheticResidueCount:0}
  ]}'
write_gate physical-air-gap \
  '["signed-offline-media","physical-network-isolation","formal-certificates","fresh-install","upgrade","rollback","air-gapped-minimum-rbac","media-handover-recorded"]' \
  '{installationPassed:true,upgradePassed:true,rollbackPassed:true}'
write_gate customer-kubernetes \
  '["customer-cni","customer-csi","customer-ingress","private-registry","signed-image-verification","schema113-migration","rolling-upgrade","application-rollback","zero-probe-failures"]' \
  '{upgradeProbeFailures:0,rollbackProbeFailures:0,readyReplicasAfterRollback:2}'
write_gate remote-signing-hsm \
  '["non-exportable-key","pin-session-control","ha-failover","audit-correlation","rate-limit-enforced","performance-target-met","final-apk-certificate-verified","zero-secret-in-control-plane"]' \
  '{requests:1000,failedRequests:0,observedP99Milliseconds:80,targetP99Milliseconds:100,haFailoverSeconds:2,targetHaFailoverSeconds:10}'
write_gate observability-egress \
  '["final-allowlist","dns","firewall","connect-proxy-capacity","prometheus-scrape","otlp-trace","siem-parse","siem-route","siem-retention","siem-alert"]' \
  '{droppedAuditEvents:0,missingMetrics:0,missingTraces:0}'
write_gate disaster-recovery \
  '["approved-production-scale-data-isolation","cross-failure-domain-copy","mysql-pitr","object-version-recovery","etcd-recovery","business-digest-match","rpo-target-met","rto-target-met"]' \
  '{rpoSeconds:60,targetRpoSeconds:300,rtoSeconds:300,targetRtoSeconds:3600,restoredBusinessObjectCount:100000,digestMismatchCount:0}'
write_gate capacity-soak \
  '["customer-approved-load-profile","day-level-duration","api-zero-errors","apk-zero-errors","object-zero-errors","zero-core-restarts","throughput-targets-met","peak-targets-met"]' \
  '{actualDurationSeconds:86401,apiRequests:100000,apiErrors:0,apkBuilds:1000,apkBuildErrors:0,objectRoundTrips:10000,objectErrors:0,coreContainerRestarts:0,targetApiRps:10,observedApiRps:12,targetApkBuildsPerHour:20,observedApkBuildsPerHour:24,targetObjectMiBPerSecond:5,observedObjectMiBPerSecond:6}'
write_gate android-physical-devices \
  '["customer-approved-device-matrix","physical-device","old-version-install","old-version-launch","ui-verified","same-signature-in-place-upgrade","first-install-time-preserved","new-version-launch"]' \
  "{approvedTargets:[{targetId:\"android-29-arm64\"}],results:[{targetId:\"android-29-arm64\",deviceFingerprint:\"$site_fingerprint\",physicalDevice:true,oldVersionInstall:true,oldVersionLaunch:true,uiVerified:true,sameSignatureInPlaceUpgrade:true,firstInstallTimePreserved:true,newVersionLaunch:true}]}"

jq -n \
  --arg siteFingerprint "$site_fingerprint" \
  --arg releaseVersion "$release_version" \
  --arg targetSchemaVersion "$schema_version" \
  --arg gitCommit "$git_commit" '
  {
    environmentKind: "customer-test",
    siteFingerprint: $siteFingerprint,
    releaseVersion: $releaseVersion,
    targetSchemaVersion: $targetSchemaVersion,
    agentProtocol: 3,
    gitCommit: $gitCommit,
    generatedAt: "2026-08-18T00:01:00Z",
    customerApproval: {approvalRef:"CUSTOMER-CHANGE-TEST",approverRole:"customer-change-owner"},
    appforgeApproval: {approvalRef:"APPFORGE-DELIVERY-TEST",approverRole:"appforge-delivery-owner"}
  }
' >"$temporary/metadata.json"

"$assembler" "$gate_source" "$bundle" "$temporary/metadata.json" \
  "$temporary/customer.pub" "$temporary/appforge.pub" >/dev/null

sign_checksums() {
  openssl dgst -sha256 -sign "$temporary/customer.key" -out "$bundle/SHA256SUMS.customer.sig" "$bundle/SHA256SUMS"
  openssl dgst -sha256 -sign "$temporary/appforge.key" -out "$bundle/SHA256SUMS.appforge.sig" "$bundle/SHA256SUMS"
}

seal_bundle() {
  local customer_key_sha appforge_key_sha gates_json
  customer_key_sha=$(sha256sum "$temporary/customer.pub" | awk '{print $1}')
  appforge_key_sha=$(sha256sum "$temporary/appforge.pub" | awk '{print $1}')
  gates_json=$(
    for file in "$bundle"/*.json; do
      [[ ${file##*/} == CUSTOMER-SITE-MANIFEST.json ]] && continue
      gate_id=$(jq -r .gateId "$file")
      sha=$(sha256sum "$file" | awk '{print $1}')
      jq -nc --arg gateId "$gate_id" --arg file "${file##*/}" --arg sha256 "$sha" \
        '{gateId:$gateId,file:$file,sha256:$sha256,result:"passed"}'
    done | jq -s 'sort_by(.gateId)'
  )
  jq -n \
    --arg siteFingerprint "$site_fingerprint" \
    --arg releaseVersion "$release_version" \
    --arg targetSchemaVersion "$schema_version" \
    --arg gitCommit "$git_commit" \
    --arg customerKeySha "$customer_key_sha" \
    --arg appforgeKeySha "$appforge_key_sha" \
    --argjson gates "$gates_json" '
    {
      schemaVersion: 1,
      evidenceType: "v7-customer-site-acceptance",
      result: "passed",
      environmentKind: "customer-test",
      siteFingerprint: $siteFingerprint,
      releaseVersion: $releaseVersion,
      targetSchemaVersion: $targetSchemaVersion,
      agentProtocol: 3,
      gitCommit: $gitCommit,
      generatedAt: "2026-08-18T00:01:00Z",
      customerApproval: {status:"accepted",approvalRef:"CUSTOMER-CHANGE-TEST",approverRole:"customer-change-owner",publicKeySha256:$customerKeySha},
      appforgeApproval: {status:"accepted",approvalRef:"APPFORGE-DELIVERY-TEST",approverRole:"appforge-delivery-owner",publicKeySha256:$appforgeKeySha},
      gates: $gates
    }
  ' >"$bundle/CUSTOMER-SITE-MANIFEST.json"
  (
    cd "$bundle"
    sha256sum CUSTOMER-SITE-MANIFEST.json \
      android-physical-devices.json capacity-soak.json customer-kubernetes.json \
      customer-object-storage.json disaster-recovery.json observability-egress.json \
      physical-air-gap.json remote-signing-hsm.json >SHA256SUMS
  )
  sign_checksums
}

expect_failure() {
  local label=$1
  if "$validator" "$bundle" "$temporary/customer.pub" "$temporary/appforge.pub" >/dev/null 2>&1; then
    echo "验收失败: $label 未被拒绝" >&2
    exit 1
  fi
}

sign_checksums
"$validator" "$bundle" "$temporary/customer.pub" "$temporary/appforge.pub" >/dev/null

jq '.releaseVersion="1.3.0"' "$temporary/metadata.json" >"$temporary/version-drift-metadata.json"
if "$assembler" "$gate_source" "$temporary/version-drift-output" "$temporary/version-drift-metadata.json" \
  "$temporary/customer.pub" "$temporary/appforge.pub" >/dev/null 2>&1; then
  echo "验收失败: 非 1.2.x 客户现场证据被汇总" >&2
  exit 1
fi

jq '.environmentKind="fixture"' "$gate_source/customer-kubernetes.json" >"$temporary/gate-context-drift.json"
mv "$temporary/gate-context-drift.json" "$gate_source/customer-kubernetes.json"
if "$assembler" "$gate_source" "$temporary/gate-context-drift-output" "$temporary/metadata.json" \
  "$temporary/customer.pub" "$temporary/appforge.pub" >/dev/null 2>&1; then
  echo "验收失败: 门禁与汇总元数据环境漂移未被拒绝" >&2
  exit 1
fi
[[ ! -e $temporary/gate-context-drift-output ]] || {
  echo "验收失败: 汇总失败后遗留部分证据目录" >&2
  exit 1
}
jq '.environmentKind="customer-test"' "$gate_source/customer-kubernetes.json" >"$temporary/gate-context-restored.json"
mv "$temporary/gate-context-restored.json" "$gate_source/customer-kubernetes.json"

printf '\n' >>"$bundle/customer-object-storage.json"
expect_failure "摘要篡改"
jq -c . "$bundle/customer-object-storage.json" >"$temporary/restored.json"
mv "$temporary/restored.json" "$bundle/customer-object-storage.json"
seal_bundle

jq '.environmentKind="fixture"' "$bundle/customer-kubernetes.json" >"$temporary/mutated.json"
mv "$temporary/mutated.json" "$bundle/customer-kubernetes.json"
seal_bundle
expect_failure "fixture 冒充客户环境"
jq '.environmentKind="customer-test"' "$bundle/customer-kubernetes.json" >"$temporary/mutated.json"
mv "$temporary/mutated.json" "$bundle/customer-kubernetes.json"
seal_bundle

jq '.verified -= ["non-exportable-key"]' "$bundle/remote-signing-hsm.json" >"$temporary/mutated.json"
mv "$temporary/mutated.json" "$bundle/remote-signing-hsm.json"
seal_bundle
expect_failure "缺少 HSM 不可导出密钥门禁"
jq '.verified += ["non-exportable-key"]' "$bundle/remote-signing-hsm.json" >"$temporary/mutated.json"
mv "$temporary/mutated.json" "$bundle/remote-signing-hsm.json"
seal_bundle

jq '.metrics.endpoint="https://customer.invalid"' "$bundle/observability-egress.json" >"$temporary/mutated.json"
mv "$temporary/mutated.json" "$bundle/observability-egress.json"
seal_bundle
expect_failure "未脱敏客户端点"
jq 'del(.metrics.endpoint)' "$bundle/observability-egress.json" >"$temporary/mutated.json"
mv "$temporary/mutated.json" "$bundle/observability-egress.json"
seal_bundle

cp "$bundle/SHA256SUMS.appforge.sig" "$bundle/SHA256SUMS.customer.sig"
expect_failure "客户签名篡改"

mkdir -p "$(dirname "$report_file")"
jq -n \
  --arg acceptedAt "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg gitCommit "$(git -C "$repo_root" rev-parse HEAD)" \
  --arg os "$(uname -s | tr '[:upper:]' '[:lower:]')" \
  --arg architecture "$(uname -m)" '
  {
    schemaVersion: 1,
    evidenceType: "v7-customer-site-evidence-contract",
    acceptedAt: $acceptedAt,
    result: "passed",
    source: {gitCommit: $gitCommit},
    runtime: {os: $os, architecture: $architecture},
    verified: [
      "fixed-eight-gate-file-set",
      "customer-test-or-production-environment-only",
      "release-schema113-agent-protocol3-boundary",
      "exact-common-and-gate-specific-fields",
      "private-pending-template-layout",
      "pending-template-assembly-rejected",
      "failed-assembly-removes-partial-output",
      "customer-and-appforge-detached-signatures",
      "sha256-tamper-rejected",
      "fixture-environment-rejected",
      "missing-hsm-non-exportable-key-rejected",
      "unredacted-endpoint-rejected",
      "customer-signature-tamper-rejected",
      "offline-delivery-layout-contract-includes-assembler-validator-and-runbook"
    ],
    dataPolicy: {
      syntheticContractDataOnly: true,
      customerDataAccessed: false,
      productionCredentialsAccessed: false,
      privateSigningKeysPersisted: false
    },
    limitations: [
      "contract-fixture-not-customer-site-acceptance",
      "does-not-close-any-of-the-eight-customer-gates",
      "organizational-records-and-key-custody-require-customer-process"
    ]
  }
' >"$temporary/contract-report.json"
install -m 0600 "$temporary/contract-report.json" "$report_file"

echo "通过: 客户现场固定 8 门禁、生产边界、SHA、防敏感信息和双签证据契约"
echo "证据: $report_file"
