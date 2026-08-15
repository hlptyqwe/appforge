#!/usr/bin/env bash

set -euo pipefail

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$delivery_dir"
"$delivery_dir/preflight.sh"

if [[ -t 0 ]]; then
  read -r -s -p "一次性注册码: " registration_token
  echo
else
  IFS= read -r registration_token
fi
[[ -n ${registration_token:-} ]] || { echo "注册码不能为空" >&2; exit 1; }

printf '%s' "$registration_token" | docker compose --profile registration run --rm -T registration
registration_token=''
unset registration_token

docker compose up -d local-agent
for _ in $(seq 1 30); do
  container_id=$(docker compose ps -q local-agent)
  status=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$container_id" 2>/dev/null || true)
  [[ $status == healthy ]] && { echo "Local Agent 注册并启动成功"; exit 0; }
  [[ $status == unhealthy ]] && break
  sleep 2
done
docker compose logs --tail 80 local-agent >&2
echo "Local Agent 未在预期时间内健康" >&2
exit 1
