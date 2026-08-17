#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 3 ]] || {
  echo "用法: $0 EVIDENCE_DIRECTORY CUSTOMER_PUBLIC_KEY APPFORGE_PUBLIC_KEY" >&2
  exit 1
}

evidence=$1
customer_public_key=$2
appforge_public_key=$3
manifest_name=CUSTOMER-SITE-MANIFEST.json
checksum_name=SHA256SUMS
customer_signature_name=SHA256SUMS.customer.sig
appforge_signature_name=SHA256SUMS.appforge.sig
gate_ids=(
  customer-object-storage
  physical-air-gap
  customer-kubernetes
  remote-signing-hsm
  observability-egress
  disaster-recovery
  capacity-soak
  android-physical-devices
)

fail() {
  echo "客户现场证据校验失败: $*" >&2
  exit 1
}

for tool in jq sha256sum openssl; do
  command -v "$tool" >/dev/null 2>&1 || fail "缺少工具 $tool"
done
[[ -d $evidence && ! -L $evidence ]] || fail "证据目录不存在或为符号链接"
for key in "$customer_public_key" "$appforge_public_key"; do
  [[ -f $key && ! -L $key ]] || fail "验签公钥必须是普通文件: $key"
  openssl pkey -pubin -in "$key" -noout >/dev/null 2>&1 || fail "无效的验签公钥: $key"
done

expected_data_names=$(
  {
    printf '%s\n' "$manifest_name"
    for gate_id in "${gate_ids[@]}"; do printf '%s.json\n' "$gate_id"; done
  } | LC_ALL=C sort
)
expected_directory_names=$(
  {
    printf '%s\n' "$expected_data_names"
    printf '%s\n' "$checksum_name" "$customer_signature_name" "$appforge_signature_name"
  } | LC_ALL=C sort
)
actual_directory_names=$(find "$evidence" -mindepth 1 -maxdepth 1 -print | sed 's#^.*/##' | LC_ALL=C sort)
[[ $actual_directory_names == "$expected_directory_names" ]] || fail "目录文件集合与固定 8 门禁契约不一致"

for path in "$evidence"/*; do
  [[ -f $path && ! -L $path ]] || fail "证据目录只允许普通文件: $path"
done
actual_checksum_names=$(awk 'NF == 2 {sub(/^\*/, "", $2); print $2}' "$evidence/$checksum_name" | LC_ALL=C sort)
[[ $actual_checksum_names == "$expected_data_names" ]] || fail "SHA256SUMS 文件集合与固定契约不一致"
(cd "$evidence" && sha256sum -c "$checksum_name" >/dev/null) || fail "证据 SHA-256 校验失败"

openssl dgst -sha256 -verify "$customer_public_key" \
  -signature "$evidence/$customer_signature_name" "$evidence/$checksum_name" >/dev/null 2>&1 ||
  fail "客户 detached signature 无效"
openssl dgst -sha256 -verify "$appforge_public_key" \
  -signature "$evidence/$appforge_signature_name" "$evidence/$checksum_name" >/dev/null 2>&1 ||
  fail "AppForge detached signature 无效"

customer_key_sha=$(sha256sum "$customer_public_key" | awk '{print $1}')
appforge_key_sha=$(sha256sum "$appforge_public_key" | awk '{print $1}')
manifest="$evidence/$manifest_name"
jq -e \
  --arg customerKeySha "$customer_key_sha" \
  --arg appforgeKeySha "$appforge_key_sha" '
  (keys | sort) == (["schemaVersion","evidenceType","result","environmentKind","siteFingerprint","releaseVersion","targetSchemaVersion","agentProtocol","gitCommit","generatedAt","customerApproval","appforgeApproval","gates"] | sort) and
  (.customerApproval | keys | sort) == (["status","approvalRef","approverRole","publicKeySha256"] | sort) and
  (.appforgeApproval | keys | sort) == (["status","approvalRef","approverRole","publicKeySha256"] | sort) and
  all(.gates[]; (keys | sort) == (["gateId","file","sha256","result"] | sort)) and
  .schemaVersion == 1 and
  .evidenceType == "v7-customer-site-acceptance" and
  .result == "passed" and
  (.environmentKind == "customer-test" or .environmentKind == "production") and
  (.siteFingerprint | test("^[0-9a-f]{64}$")) and
  (.releaseVersion | test("^1\\.2\\.[0-9]+$")) and
  .targetSchemaVersion == "20260815_113_v7_air_gapped" and
  .agentProtocol == 3 and
  (.gitCommit | test("^[0-9a-f]{40}$")) and
  (.generatedAt | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$")) and
  .customerApproval.status == "accepted" and
  .customerApproval.publicKeySha256 == $customerKeySha and
  (.customerApproval.approvalRef | test("^[A-Za-z0-9._-]{1,128}$")) and
  (.customerApproval.approverRole | test("^[A-Za-z0-9._-]{1,128}$")) and
  .appforgeApproval.status == "accepted" and
  .appforgeApproval.publicKeySha256 == $appforgeKeySha and
  (.appforgeApproval.approvalRef | test("^[A-Za-z0-9._-]{1,128}$")) and
  (.appforgeApproval.approverRole | test("^[A-Za-z0-9._-]{1,128}$")) and
  (.gates | type == "array" and length == 8)
' "$manifest" >/dev/null || fail "总清单元数据、生产边界或双签批准信息无效"
if jq -e '[.. | strings | select(test("://|-----BEGIN|AKIA[0-9A-Z]{16}"))] | length > 0' "$manifest" >/dev/null; then
  fail "总清单包含未脱敏端点或密钥材料"
fi

manifest_context=$(jq -r '[.environmentKind,.siteFingerprint,.releaseVersion,.targetSchemaVersion,(.agentProtocol|tostring),.gitCommit] | @tsv' "$manifest")

required_tokens() {
  case "$1" in
    customer-object-storage)
      printf '%s\n' aws-s3 aliyun-oss least-privilege-prefix workload-identity-or-short-lived-credential put-stat-full-get-sha256-delete-not-found existing-objects-unchanged zero-synthetic-residue
      ;;
    physical-air-gap)
      printf '%s\n' signed-offline-media physical-network-isolation formal-certificates fresh-install upgrade rollback air-gapped-minimum-rbac media-handover-recorded
      ;;
    customer-kubernetes)
      printf '%s\n' customer-cni customer-csi customer-ingress private-registry signed-image-verification schema113-migration rolling-upgrade application-rollback zero-probe-failures
      ;;
    remote-signing-hsm)
      printf '%s\n' non-exportable-key pin-session-control ha-failover audit-correlation rate-limit-enforced performance-target-met final-apk-certificate-verified zero-secret-in-control-plane
      ;;
    observability-egress)
      printf '%s\n' final-allowlist dns firewall connect-proxy-capacity prometheus-scrape otlp-trace siem-parse siem-route siem-retention siem-alert
      ;;
    disaster-recovery)
      printf '%s\n' approved-production-scale-data-isolation cross-failure-domain-copy mysql-pitr object-version-recovery etcd-recovery business-digest-match rpo-target-met rto-target-met
      ;;
    capacity-soak)
      printf '%s\n' customer-approved-load-profile day-level-duration api-zero-errors apk-zero-errors object-zero-errors zero-core-restarts throughput-targets-met peak-targets-met
      ;;
    android-physical-devices)
      printf '%s\n' customer-approved-device-matrix physical-device old-version-install old-version-launch ui-verified same-signature-in-place-upgrade first-install-time-preserved new-version-launch
      ;;
    *) fail "未知门禁 $1" ;;
  esac
}

for gate_id in "${gate_ids[@]}"; do
  file_name="$gate_id.json"
  file="$evidence/$file_name"
  file_sha=$(sha256sum "$file" | awk '{print $1}')
  [[ $(jq -r --arg gate "$gate_id" '[.gates[] | select(.gateId == $gate)] | length' "$manifest") == 1 ]] ||
    fail "总清单缺少或重复门禁: $gate_id"
  jq -e --arg gate "$gate_id" --arg file "$file_name" --arg sha "$file_sha" '
    [.gates[] | select(.gateId == $gate and .file == $file and .sha256 == $sha and .result == "passed")] | length == 1
  ' "$manifest" >/dev/null || fail "总清单门禁文件或摘要不匹配: $gate_id"

  file_context=$(jq -r '[.environmentKind,.siteFingerprint,.releaseVersion,.targetSchemaVersion,(.agentProtocol|tostring),.gitCommit] | @tsv' "$file")
  [[ $file_context == "$manifest_context" ]] || fail "门禁与总清单环境/版本/Schema/协议/提交不一致: $gate_id"
  [[ $(jq -r .generatedAt "$manifest") > $(jq -r .finishedAt "$file") || $(jq -r .generatedAt "$manifest") == $(jq -r .finishedAt "$file") ]] ||
    fail "总清单生成时间早于门禁结束时间: $gate_id"
  jq -e --arg gate "$gate_id" '
    (keys | sort) == (["schemaVersion","evidenceType","gateId","result","runId","environmentKind","siteFingerprint","releaseVersion","targetSchemaVersion","agentProtocol","gitCommit","startedAt","finishedAt","verified","metrics","openFindings","dataPolicy"] | sort) and
    (.dataPolicy | keys | sort) == (["credentialsIncluded","rawCustomerDataIncluded","executionApprovalRef"] | sort) and
    .schemaVersion == 1 and
    .evidenceType == "v7-customer-site-gate" and
    .gateId == $gate and
    .result == "passed" and
    (.runId | type == "string" and test("^[A-Za-z0-9._:-]{8,128}$")) and
    (.startedAt | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$")) and
    (.finishedAt | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$")) and
    .finishedAt >= .startedAt and
    (.verified | type == "array" and length > 0 and all(type == "string")) and
    (.openFindings | type == "array" and length == 0) and
    .dataPolicy.credentialsIncluded == false and
    .dataPolicy.rawCustomerDataIncluded == false and
    (.dataPolicy.executionApprovalRef | test("^[A-Za-z0-9._-]{1,128}$"))
  ' "$file" >/dev/null || fail "门禁公共字段无效或仍有开放问题: $gate_id"

  required_json=$(required_tokens "$gate_id" | jq -R . | jq -s 'sort')
  jq -e --argjson required "$required_json" '(.verified | sort) == $required' "$file" >/dev/null ||
    fail "$gate_id 的 verified 必须与固定契约完全一致"
  while IFS= read -r token; do
    jq -e --arg token "$token" '.verified | index($token) != null' "$file" >/dev/null ||
      fail "$gate_id 缺少验证项: $token"
  done < <(required_tokens "$gate_id")

  if jq -e '
    [paths(scalars) as $path |
      ($path[-1] | tostring | ascii_downcase) |
      select(test("password|secret|bearer|private.?key|access.?key|session.?key|keystore"))] | length > 0
  ' "$file" >/dev/null; then
    fail "$gate_id 包含禁止的敏感字段名"
  fi
  if jq -e '[.. | strings | select(test("://|-----BEGIN|AKIA[0-9A-Z]{16}"))] | length > 0' "$file" >/dev/null; then
    fail "$gate_id 包含未脱敏端点、密钥材料或访问密钥"
  fi
done

jq -e '
  . as $root |
  ($root.metrics | keys) == ["providerResults"] and
  ($root.metrics.providerResults | type == "array" and length == 2) and
  ([$root.metrics.providerResults[] | .provider] | sort) == ["aliyun-oss", "aws-s3"] and
  all($root.metrics.providerResults[]; (keys | sort) == (["provider","passed","leastPrivilegePrefix","fullReadbackSha256","deleteNotFoundConfirmed","existingObjectsUnchanged","syntheticResidueCount"] | sort)) and
  all($root.metrics.providerResults[]; .passed == true and .leastPrivilegePrefix == true and .fullReadbackSha256 == true and
      .deleteNotFoundConfirmed == true and .existingObjectsUnchanged == true and .syntheticResidueCount == 0)
' "$evidence/customer-object-storage.json" >/dev/null || fail "客户对象存储必须同时通过真实 AWS S3 与阿里云 OSS"

jq -e '
  (.metrics | keys | sort) == (["installationPassed","upgradePassed","rollbackPassed"] | sort) and
  .metrics.installationPassed == true and .metrics.upgradePassed == true and .metrics.rollbackPassed == true
' "$evidence/physical-air-gap.json" >/dev/null || fail "客户物理断网安装、升级或回滚未全部通过"

jq -e '
  (.metrics | keys | sort) == (["upgradeProbeFailures","rollbackProbeFailures","readyReplicasAfterRollback"] | sort) and
  .metrics.upgradeProbeFailures == 0 and .metrics.rollbackProbeFailures == 0 and
  .metrics.readyReplicasAfterRollback >= 2
' "$evidence/customer-kubernetes.json" >/dev/null || fail "客户 Kubernetes 升级/回滚指标未达门禁"

jq -e '
  (.metrics | keys | sort) == (["requests","failedRequests","observedP99Milliseconds","targetP99Milliseconds","haFailoverSeconds","targetHaFailoverSeconds"] | sort) and
  .metrics.requests > 0 and .metrics.failedRequests == 0 and
  .metrics.observedP99Milliseconds > 0 and
  .metrics.observedP99Milliseconds <= .metrics.targetP99Milliseconds and
  .metrics.haFailoverSeconds >= 0 and .metrics.haFailoverSeconds <= .metrics.targetHaFailoverSeconds
' "$evidence/remote-signing-hsm.json" >/dev/null || fail "客户 HSM 可用性或性能指标未达门禁"

jq -e '
  (.metrics | keys | sort) == (["droppedAuditEvents","missingMetrics","missingTraces"] | sort) and
  .metrics.droppedAuditEvents == 0 and .metrics.missingMetrics == 0 and .metrics.missingTraces == 0
' "$evidence/observability-egress.json" >/dev/null || fail "客户可观测性或审计出口存在丢失"

jq -e '
  (.metrics | keys | sort) == (["rpoSeconds","targetRpoSeconds","rtoSeconds","targetRtoSeconds","restoredBusinessObjectCount","digestMismatchCount"] | sort) and
  .metrics.rpoSeconds >= 0 and .metrics.rpoSeconds <= .metrics.targetRpoSeconds and
  .metrics.rtoSeconds >= 0 and .metrics.rtoSeconds <= .metrics.targetRtoSeconds and
  .metrics.restoredBusinessObjectCount > 0 and .metrics.digestMismatchCount == 0
' "$evidence/disaster-recovery.json" >/dev/null || fail "客户灾备 RPO/RTO 或业务摘要未达门禁"

jq -e '
  (.metrics | keys | sort) == (["actualDurationSeconds","apiRequests","apiErrors","apkBuilds","apkBuildErrors","objectRoundTrips","objectErrors","coreContainerRestarts","targetApiRps","observedApiRps","targetApkBuildsPerHour","observedApkBuildsPerHour","targetObjectMiBPerSecond","observedObjectMiBPerSecond"] | sort) and
  .metrics.actualDurationSeconds >= 86400 and
  .metrics.apiRequests > 0 and .metrics.apiErrors == 0 and
  .metrics.apkBuilds > 0 and .metrics.apkBuildErrors == 0 and
  .metrics.objectRoundTrips > 0 and .metrics.objectErrors == 0 and
  .metrics.coreContainerRestarts == 0 and
  .metrics.targetApiRps > 0 and .metrics.targetApkBuildsPerHour > 0 and .metrics.targetObjectMiBPerSecond > 0 and
  .metrics.observedApiRps >= .metrics.targetApiRps and
  .metrics.observedApkBuildsPerHour >= .metrics.targetApkBuildsPerHour and
  .metrics.observedObjectMiBPerSecond >= .metrics.targetObjectMiBPerSecond
' "$evidence/capacity-soak.json" >/dev/null || fail "客户天级容量/峰值指标未达门禁"

jq -e '
  . as $root |
  ($root.metrics | keys | sort) == (["approvedTargets","results"] | sort) and
  ($root.metrics.approvedTargets | type == "array" and length > 0) and
  ($root.metrics.results | type == "array") and
  (($root.metrics.results | length) == ($root.metrics.approvedTargets | length)) and
  all($root.metrics.approvedTargets[]; (keys | sort) == ["targetId"] and (.targetId | test("^[A-Za-z0-9._-]{1,128}$"))) and
  ([$root.metrics.approvedTargets[].targetId] | sort) == ([$root.metrics.results[].targetId] | sort) and
  all($root.metrics.results[];
    (keys | sort) == (["targetId","deviceFingerprint","physicalDevice","oldVersionInstall","oldVersionLaunch","uiVerified","sameSignatureInPlaceUpgrade","firstInstallTimePreserved","newVersionLaunch"] | sort) and
    (.deviceFingerprint | test("^[0-9a-f]{64}$")) and .physicalDevice == true and
    .oldVersionInstall == true and .oldVersionLaunch == true and .uiVerified == true and
    .sameSignatureInPlaceUpgrade == true and .firstInstallTimePreserved == true and
    .newVersionLaunch == true)
' "$evidence/android-physical-devices.json" >/dev/null || fail "客户批准的 Android 物理真机矩阵未全部通过"

echo "客户现场 V7 证据通过固定 8 门禁、SHA-256 和双 detached signature 校验"
