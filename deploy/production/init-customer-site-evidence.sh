#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 5 ]] || {
  echo "用法: $0 OUTPUT_DIRECTORY RELEASE_VERSION GIT_COMMIT SITE_FINGERPRINT ENVIRONMENT_KIND" >&2
  exit 1
}

output=$1
release_version=$2
git_commit=$3
site_fingerprint=$4
environment_kind=$5
schema_version=20260815_113_v7_air_gapped
generated_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

fail() {
  echo "客户现场证据模板初始化失败: $*" >&2
  exit 1
}

command -v jq >/dev/null 2>&1 || fail "缺少工具 jq"
[[ ! -e $output ]] || fail "输出目录已存在，拒绝覆盖: $output"
[[ $release_version =~ ^1\.2\.[0-9]+$ ]] || fail "客户现场验收版本必须属于 1.2.x"
[[ $git_commit =~ ^[0-9a-f]{40}$ ]] || fail "Git commit 必须是 40 位小写 SHA"
[[ $site_fingerprint =~ ^[0-9a-f]{64}$ ]] || fail "站点标识必须是脱敏后的 64 位小写 SHA-256"
[[ $environment_kind == customer-test || $environment_kind == production ]] ||
  fail "环境只允许 customer-test 或 production"

mkdir -m 700 "$output"
cleanup_output=true
cleanup() {
  if [[ $cleanup_output == true && -d $output ]]; then
    rm -rf -- "$output"
  fi
}
trap cleanup EXIT
umask 077
mkdir -m 700 "$output/gates"

jq -n \
  --arg environmentKind "$environment_kind" \
  --arg siteFingerprint "$site_fingerprint" \
  --arg releaseVersion "$release_version" \
  --arg targetSchemaVersion "$schema_version" \
  --arg gitCommit "$git_commit" \
  --arg generatedAt "$generated_at" '
  {
    environmentKind: $environmentKind,
    siteFingerprint: $siteFingerprint,
    releaseVersion: $releaseVersion,
    targetSchemaVersion: $targetSchemaVersion,
    agentProtocol: 3,
    gitCommit: $gitCommit,
    generatedAt: $generatedAt,
    customerApproval: {approvalRef:"PENDING",approverRole:"PENDING"},
    appforgeApproval: {approvalRef:"PENDING",approverRole:"PENDING"}
  }
' >"$output/metadata.json"

write_gate() {
  local gate_id=$1 metrics=$2 metrics_json
  metrics_json=$(jq -nc "$metrics")
  jq -n \
    --arg gateId "$gate_id" \
    --arg environmentKind "$environment_kind" \
    --arg siteFingerprint "$site_fingerprint" \
    --arg releaseVersion "$release_version" \
    --arg targetSchemaVersion "$schema_version" \
    --arg gitCommit "$git_commit" \
    --arg timestamp "$generated_at" \
    --argjson metrics "$metrics_json" '
    {
      schemaVersion: 1,
      evidenceType: "v7-customer-site-gate",
      gateId: $gateId,
      result: "pending",
      runId: ("PENDING-" + $gateId),
      environmentKind: $environmentKind,
      siteFingerprint: $siteFingerprint,
      releaseVersion: $releaseVersion,
      targetSchemaVersion: $targetSchemaVersion,
      agentProtocol: 3,
      gitCommit: $gitCommit,
      startedAt: $timestamp,
      finishedAt: $timestamp,
      verified: [],
      metrics: $metrics,
      openFindings: ["pending-customer-execution"],
      dataPolicy: {
        credentialsIncluded: false,
        rawCustomerDataIncluded: false,
        executionApprovalRef: "PENDING"
      }
    }
  ' >"$output/gates/$gate_id.json"
}

write_gate customer-object-storage '{providerResults:[
  {provider:"aws-s3",passed:false,leastPrivilegePrefix:false,fullReadbackSha256:false,deleteNotFoundConfirmed:false,existingObjectsUnchanged:false,syntheticResidueCount:null},
  {provider:"aliyun-oss",passed:false,leastPrivilegePrefix:false,fullReadbackSha256:false,deleteNotFoundConfirmed:false,existingObjectsUnchanged:false,syntheticResidueCount:null}
]}'
write_gate physical-air-gap '{installationPassed:false,upgradePassed:false,rollbackPassed:false}'
write_gate customer-kubernetes '{upgradeProbeFailures:null,rollbackProbeFailures:null,readyReplicasAfterRollback:null}'
write_gate remote-signing-hsm '{requests:null,failedRequests:null,observedP99Milliseconds:null,targetP99Milliseconds:null,haFailoverSeconds:null,targetHaFailoverSeconds:null}'
write_gate observability-egress '{droppedAuditEvents:null,missingMetrics:null,missingTraces:null}'
write_gate disaster-recovery '{rpoSeconds:null,targetRpoSeconds:null,rtoSeconds:null,targetRtoSeconds:null,restoredBusinessObjectCount:null,digestMismatchCount:null}'
write_gate capacity-soak '{actualDurationSeconds:null,apiRequests:null,apiErrors:null,apkBuilds:null,apkBuildErrors:null,objectRoundTrips:null,objectErrors:null,coreContainerRestarts:null,targetApiRps:null,observedApiRps:null,targetApkBuildsPerHour:null,observedApkBuildsPerHour:null,targetObjectMiBPerSecond:null,observedObjectMiBPerSecond:null}'
write_gate android-physical-devices '{approvedTargets:[],results:[]}'

cleanup_output=false
trap - EXIT
echo "客户现场证据 pending 模板已生成: $output"
echo "模板不能通过汇总/双签门禁；只有完成现场执行、填入精确 verified/metrics 并清空 openFindings 后才能汇总"
