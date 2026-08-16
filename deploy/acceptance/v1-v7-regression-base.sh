#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
go_cache=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache}
go_mod_cache=${APPFORGE_V7_GO_MOD_CACHE:-/private/tmp/appforge-go-mod-cache}
node_bin_dir=${APPFORGE_NODE_BIN_DIR:-}
evidence=${APPFORGE_BASE_REGRESSION_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v1-v7-base-regression-20260815.json}
regression_tmp=$(mktemp -d)
trap 'rm -rf "$regression_tmp"' EXIT

if [[ -z $node_bin_dir ]]; then
  node_home=$(node -p 'require("os").homedir()' 2>/dev/null || true)
  for candidate in "$node_home"/.nvm/versions/node/v20.*/bin; do
    [[ -x $candidate/node ]] && node_bin_dir=$candidate
  done
fi
if [[ -n $node_bin_dir ]]; then
  PATH="$node_bin_dir:$PATH"
  export PATH
fi
node_major=$(node -p 'Number(process.versions.node.split(".")[0])')
[[ $node_major -ge 20 ]] || {
  echo "验收失败: 前端回归要求 Node.js >=20；可通过 APPFORGE_NODE_BIN_DIR 指定 bin 目录" >&2
  exit 1
}

go_modules=(
  common
  proto/common
  proto/system
  proto/core
  proto/builder
  services/system
  services/core
  services/builder
  appforge-api
  local-agent
  appforgectl
)

for module in "${go_modules[@]}"; do
  echo "回归: go test ./$module/..."
  (
    cd "$repo_root/$module"
    GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./... -count=1
  )
done
echo "通过: 11 个 Go 模块全量单元/集成测试与编译检查"

(
  cd "$repo_root/appforge-ui"
  npm run type-check
)
echo "通过: 现有统一前端 TypeScript/Vue 类型检查 (Node $(node --version))"

while IFS= read -r -d '' script; do
  bash -n "$script"
done < <(find "$repo_root/deploy/acceptance" "$repo_root/deploy/production" "$repo_root/deploy/local-agent" \
  -type f -name '*.sh' -print0)
echo "通过: 验收、生产交付和 Local Agent Shell 语法检查"

APPFORGE_V2_EVIDENCE_PATH="$regression_tmp/v2-branding-readonly.json" \
  "$repo_root/deploy/acceptance/v2-branding-readonly.sh"
APPFORGE_V2_FIXTURE_ONLY=true \
  "$repo_root/deploy/acceptance/v2-branding-negative-e2e.sh"
if [[ ${APPFORGE_RUN_V2_NEGATIVE_E2E:-false} == true ]]; then
  [[ ${APPFORGE_V2_ALLOW_WRITE_E2E:-false} == true ]] || {
    echo "验收失败: 启用 V2 负向 E2E 时必须同时设置 APPFORGE_V2_ALLOW_WRITE_E2E=true" >&2
    exit 1
  }
  APPFORGE_V2_NEGATIVE_EVIDENCE_PATH="$regression_tmp/v2-branding-negative-e2e.json" \
    "$repo_root/deploy/acceptance/v2-branding-negative-e2e.sh"
else
  echo "未执行: V2 负向写入 E2E 需要显式设置 APPFORGE_RUN_V2_NEGATIVE_E2E=true 和 APPFORGE_V2_ALLOW_WRITE_E2E=true"
fi
APPFORGE_V3_EVIDENCE_PATH="$regression_tmp/v3-whitelabel-readonly.json" \
  "$repo_root/deploy/acceptance/v3-whitelabel-readonly.sh"
"$repo_root/deploy/acceptance/v4-builder-recovery.sh"
"$repo_root/deploy/acceptance/v4-build-cluster.sh"
"$repo_root/deploy/acceptance/v5-open-platform.sh"
"$repo_root/deploy/acceptance/v6-commercialization.sh"
"$repo_root/deploy/acceptance/v7-enterprise-delivery.sh"
"$repo_root/deploy/acceptance/v7-performance-smoke.sh"

if [[ ${APPFORGE_RUN_REMOTE_SIGNING_TASK_E2E:-false} == true ]]; then
  APPFORGE_REMOTE_SIGNER_TASK_EVIDENCE_PATH="$regression_tmp/v7-remote-signing-task.json" \
    "$repo_root/deploy/acceptance/v7-remote-signing-task-e2e.sh"
else
  echo "未执行: V7 远程签名完整任务 E2E 需要显式设置 APPFORGE_RUN_REMOTE_SIGNING_TASK_E2E=true"
fi

if [[ ${APPFORGE_RUN_ANDROID_RUNTIME:-false} == true ]]; then
  : "${APPFORGE_V3_ANDROID_OLD_APK:?启用 Android 运行时验收时必须指定旧版 APK}"
  : "${APPFORGE_V3_ANDROID_NEW_APK:?启用 Android 运行时验收时必须指定新版 APK}"
  "$repo_root/deploy/acceptance/v3-android-runtime.sh" \
    "$APPFORGE_V3_ANDROID_OLD_APK" \
    "$APPFORGE_V3_ANDROID_NEW_APK" \
    "${APPFORGE_V3_ANDROID_PACKAGE:-com.appforge.v3runtime20260814}" \
    "${APPFORGE_V3_ANDROID_ACTIVITY:-com.appforge.acceptance.MainActivity}" \
    "${APPFORGE_V3_ANDROID_OLD_VERSION_CODE:-20260820}" \
    "${APPFORGE_V3_ANDROID_NEW_VERSION_CODE:-20260821}" \
    "${APPFORGE_V3_ANDROID_CERTIFICATE_SHA256:-59133f9c597d6968f37195a6b39330c0bef61a96be5faa7b98bedf0f60170d82}"
else
  echo "未执行: V3 Android 真机运行时验收需要显式设置 APPFORGE_RUN_ANDROID_RUNTIME=true 并连接授权设备"
fi

mkdir -p "$(dirname "$evidence")"
umask 077
EVIDENCE_PATH="$evidence" NODE_VERSION="$(node --version)" \
  V2_WRITE_E2E="${APPFORGE_RUN_V2_NEGATIVE_E2E:-false}" \
  REMOTE_SIGNING_TASK_E2E="${APPFORGE_RUN_REMOTE_SIGNING_TASK_E2E:-false}" \
  ANDROID_RUNTIME="${APPFORGE_RUN_ANDROID_RUNTIME:-false}" python3 - <<'PY'
import datetime,json,os,pathlib

payload = {
  'schemaVersion': 2,
  'evidenceType': 'v1-v7-base-regression',
  'executedAt': datetime.datetime.now(datetime.timezone.utc).astimezone().isoformat(timespec='seconds'),
  'status': 'passed',
  'command': 'deploy/acceptance/v1-v7-regression-base.sh',
  'coverage': {
    'goModules': 11,
    'frontend': f'Vue/TypeScript type-check on Node {os.environ["NODE_VERSION"]}',
    'shell': 'acceptance, production delivery and Local Agent scripts',
    'runtimeAcceptance': ['V2 read-only and no-write negative fixture', 'V3 read-only', 'V4 build cluster', 'V5 open platform', 'V6 commercialization', 'V7 enterprise delivery'],
    'performance': 'bounded health and authenticated deployment-status concurrency smoke',
    'v2WriteNegativeE2E': os.environ['V2_WRITE_E2E'] == 'true',
    'remoteSigningTaskE2E': os.environ['REMOTE_SIGNING_TASK_E2E'] == 'true',
    'androidRuntime': os.environ['ANDROID_RUNTIME'] == 'true',
  },
  'checks': {
    'goModules': 'passed',
    'frontendTypeCheck': 'passed',
    'shellSyntax': 'passed',
    'v2NoWriteFixture': 'passed',
    'v4CurrentRuntime': 'passed',
    'v5CurrentRuntime': 'passed',
    'v6CurrentRuntime': 'passed',
    'v7BaseRuntime': 'passed',
    'releaseEvidenceContract': 'passed',
    'performanceSmoke': 'passed',
  },
  'limitations': [
    'V2 public-API-to-real-Worker incompatible APK write E2E was not executed.' if os.environ['V2_WRITE_E2E'] != 'true' else None,
    'V7 isolated remote-signing full task E2E was not executed.' if os.environ['REMOTE_SIGNING_TASK_E2E'] != 'true' else None,
    'V3 Android device runtime acceptance was not executed.' if os.environ['ANDROID_RUNTIME'] != 'true' else None,
    'Customer capacity, peak-load, production-scale object throughput and long-duration stability are separate gates.',
    'Real AWS/S3/OSS, customer SIEM, physical air gap and real tag/OIDC/vulnerability-database release execution remain separate V7 gates.',
  ],
}
payload['limitations'] = [item for item in payload['limitations'] if item]
path = pathlib.Path(os.environ['EVIDENCE_PATH'])
path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + '\n')
PY
chmod 0600 "$evidence"

echo "V1-V7 基础回归通过；Android真机、客户容量/峰值/长稳、三种Artifact客户环境和外部系统验收不在本脚本完成范围内。"
echo "证据: $evidence"
