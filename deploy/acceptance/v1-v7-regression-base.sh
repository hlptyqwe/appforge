#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
go_cache=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache}
go_mod_cache=${APPFORGE_V7_GO_MOD_CACHE:-/private/tmp/appforge-go-mod-cache}
node_bin_dir=${APPFORGE_NODE_BIN_DIR:-}

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

"$repo_root/deploy/acceptance/v4-builder-recovery.sh"
"$repo_root/deploy/acceptance/v4-build-cluster.sh"
"$repo_root/deploy/acceptance/v5-open-platform.sh"
"$repo_root/deploy/acceptance/v6-commercialization.sh"
"$repo_root/deploy/acceptance/v7-enterprise-delivery.sh"
"$repo_root/deploy/acceptance/v7-performance-smoke.sh"

if [[ ${APPFORGE_RUN_ANDROID_RUNTIME:-false} == true ]]; then
  "$repo_root/deploy/acceptance/v3-android-runtime.sh"
else
  echo "未执行: V3 Android 真机运行时验收需要显式设置 APPFORGE_RUN_ANDROID_RUNTIME=true 并连接授权设备"
fi

echo "V1-V7 基础回归通过；Android真机、性能压测、三种Artifact E2E和外部系统验收不在本脚本完成范围内。"
