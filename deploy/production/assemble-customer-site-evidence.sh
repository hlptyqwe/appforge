#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 5 ]] || {
  echo "用法: $0 GATE_DIRECTORY OUTPUT_DIRECTORY METADATA_JSON CUSTOMER_PUBLIC_KEY APPFORGE_PUBLIC_KEY" >&2
  exit 1
}

gate_directory=$1
output=$2
metadata=$3
customer_public_key=$4
appforge_public_key=$5
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
  echo "客户现场证据汇总失败: $*" >&2
  exit 1
}

for tool in jq sha256sum openssl; do
  command -v "$tool" >/dev/null 2>&1 || fail "缺少工具 $tool"
done
[[ -d $gate_directory && ! -L $gate_directory ]] || fail "门禁证据目录不存在或为符号链接"
[[ -f $metadata && ! -L $metadata ]] || fail "元数据必须是普通 JSON 文件"
[[ ! -e $output ]] || fail "输出目录已存在，拒绝覆盖: $output"
for key in "$customer_public_key" "$appforge_public_key"; do
  [[ -f $key && ! -L $key ]] || fail "验签公钥必须是普通文件: $key"
  openssl pkey -pubin -in "$key" -noout >/dev/null 2>&1 || fail "无效的验签公钥: $key"
done

jq -e '
  (.environmentKind == "customer-test" or .environmentKind == "production") and
  (.siteFingerprint | test("^[0-9a-f]{64}$")) and
  (.releaseVersion | test("^1\\.2\\.[0-9]+$")) and
  .targetSchemaVersion == "20260815_113_v7_air_gapped" and
  .agentProtocol == 3 and
  (.gitCommit | test("^[0-9a-f]{40}$")) and
  (.generatedAt | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$")) and
  (.customerApproval.approvalRef | type == "string" and length >= 1 and length <= 128) and
  (.customerApproval.approverRole | type == "string" and length >= 1 and length <= 128) and
  (.appforgeApproval.approvalRef | type == "string" and length >= 1 and length <= 128) and
  (.appforgeApproval.approverRole | type == "string" and length >= 1 and length <= 128) and
  ([paths(scalars) as $path |
    ($path[-1] | tostring | ascii_downcase) |
    select(test("password|secret|bearer|private.?key|access.?key|session.?key|keystore"))] | length == 0) and
  ([.. | strings | select(test("://|-----BEGIN|AKIA[0-9A-Z]{16}"))] | length == 0)
' "$metadata" >/dev/null || fail "元数据无效、生产边界漂移或包含敏感信息"

expected_names=$(for gate_id in "${gate_ids[@]}"; do printf '%s.json\n' "$gate_id"; done | LC_ALL=C sort)
actual_names=$(find "$gate_directory" -mindepth 1 -maxdepth 1 -print | sed 's#^.*/##' | LC_ALL=C sort)
[[ $actual_names == "$expected_names" ]] || fail "输入目录必须且只能包含固定 8 个门禁 JSON"

mkdir -m 700 "$output"
trap 'rm -rf "$output"' ERR
for gate_id in "${gate_ids[@]}"; do
  source_file="$gate_directory/$gate_id.json"
  [[ -f $source_file && ! -L $source_file ]] || fail "门禁证据必须是普通文件: $gate_id"
  jq -e --arg gate "$gate_id" '
    .schemaVersion == 1 and .evidenceType == "v7-customer-site-gate" and
    .gateId == $gate and .result == "passed"
  ' "$source_file" >/dev/null || fail "门禁证据类型、ID 或结果无效: $gate_id"
  install -m 0600 "$source_file" "$output/$gate_id.json"
done

customer_key_sha=$(sha256sum "$customer_public_key" | awk '{print $1}')
appforge_key_sha=$(sha256sum "$appforge_public_key" | awk '{print $1}')
gates_json=$(
  for gate_id in "${gate_ids[@]}"; do
    file="$output/$gate_id.json"
    sha=$(sha256sum "$file" | awk '{print $1}')
    jq -nc --arg gateId "$gate_id" --arg file "$gate_id.json" --arg sha256 "$sha" \
      '{gateId:$gateId,file:$file,sha256:$sha256,result:"passed"}'
  done | jq -s 'sort_by(.gateId)'
)

jq \
  --arg customerKeySha "$customer_key_sha" \
  --arg appforgeKeySha "$appforge_key_sha" \
  --argjson gates "$gates_json" '
  {
    schemaVersion: 1,
    evidenceType: "v7-customer-site-acceptance",
    result: "passed",
    environmentKind,
    siteFingerprint,
    releaseVersion,
    targetSchemaVersion,
    agentProtocol,
    gitCommit,
    generatedAt,
    customerApproval: {
      status: "accepted",
      approvalRef: .customerApproval.approvalRef,
      approverRole: .customerApproval.approverRole,
      publicKeySha256: $customerKeySha
    },
    appforgeApproval: {
      status: "accepted",
      approvalRef: .appforgeApproval.approvalRef,
      approverRole: .appforgeApproval.approverRole,
      publicKeySha256: $appforgeKeySha
    },
    gates: $gates
  }
' "$metadata" >"$output/CUSTOMER-SITE-MANIFEST.json"

(
  cd "$output"
  sha256sum CUSTOMER-SITE-MANIFEST.json \
    android-physical-devices.json capacity-soak.json customer-kubernetes.json \
    customer-object-storage.json disaster-recovery.json observability-egress.json \
    physical-air-gap.json remote-signing-hsm.json >SHA256SUMS
)

trap - ERR
echo "客户现场证据待签名目录已生成: $output"
echo "客户和 AppForge 必须分别对 $output/SHA256SUMS 生成 detached signature，之后再运行 validate-customer-site-evidence"
