#!/usr/bin/env bash

set -euo pipefail

offline=false
if [[ $# -eq 2 && $1 == --drained ]]; then
  new_image=$2
elif [[ $# -eq 3 && $1 == --drained && $2 == --offline ]]; then
  offline=true
  new_image=$3
else
  echo "用法: $0 --drained [--offline] NEW_IMAGE" >&2
  exit 1
fi
[[ $new_image == *:* && ${new_image##*:} != latest ]] || { echo "新镜像必须使用非 latest 固定版本标签" >&2; exit 1; }

health_attempts=${APPFORGE_LOCAL_AGENT_UPGRADE_HEALTH_ATTEMPTS:-60}
health_interval=${APPFORGE_LOCAL_AGENT_UPGRADE_HEALTH_INTERVAL_SECONDS:-2}
[[ $health_attempts =~ ^[1-9][0-9]*$ && $health_interval =~ ^[1-9][0-9]*$ ]] || {
  echo "升级健康检查次数和间隔必须是正整数" >&2
  exit 1
}

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$delivery_dir"
"$delivery_dir/preflight.sh"

container_id=$(docker compose ps -q local-agent)
[[ -n $container_id ]] || { echo "Local Agent 尚未运行" >&2; exit 1; }
old_image=$(docker inspect -f '{{.Config.Image}}' "$container_id")
[[ $old_image != "$new_image" ]] || { echo "当前已经运行目标镜像"; exit 0; }

if [[ $offline == true ]]; then
  docker image inspect "$new_image" >/dev/null 2>&1 || {
    echo "离线升级镜像尚未导入: $new_image" >&2
    exit 1
  }
else
  APPFORGE_LOCAL_AGENT_IMAGE="$new_image" docker compose pull local-agent
fi
APPFORGE_LOCAL_AGENT_IMAGE="$new_image" docker compose run --rm --no-deps local-agent version | grep -Eq 'protocol=3$'
APPFORGE_LOCAL_AGENT_IMAGE="$new_image" docker compose up -d --no-deps local-agent

wait_for_health() {
  local image=$1 container_id status
  for _ in $(seq 1 "$health_attempts"); do
    container_id=$(APPFORGE_LOCAL_AGENT_IMAGE="$image" docker compose ps -q local-agent)
    status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container_id" 2>/dev/null || true)
    [[ $status == healthy ]] && return 0
    [[ $status == unhealthy ]] && return 1
    sleep "$health_interval"
  done
  return 1
}

if wait_for_health "$new_image"; then
  container_id=$(APPFORGE_LOCAL_AGENT_IMAGE="$new_image" docker compose ps -q local-agent)
  umask 077
  awk -v image="$new_image" '
    BEGIN { updated=0 }
    /^APPFORGE_LOCAL_AGENT_IMAGE=/ { print "APPFORGE_LOCAL_AGENT_IMAGE=" image; updated=1; next }
    { print }
    END { if (!updated) exit 2 }
  ' .env >.env.tmp
  mv .env.tmp .env
  echo "Local Agent 已升级: $old_image -> $new_image"
  exit 0
fi

echo "新版本不健康，自动恢复旧镜像 $old_image" >&2
APPFORGE_LOCAL_AGENT_IMAGE="$old_image" docker compose up -d --no-deps local-agent
if ! wait_for_health "$old_image"; then
  echo "旧镜像恢复后仍不健康，请保持 Drain 并人工处理: $old_image" >&2
  exit 2
fi
echo "已恢复旧镜像并确认健康: $old_image" >&2
exit 1
