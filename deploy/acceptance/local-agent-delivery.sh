#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
compose_dir="$repo_root/deploy/local-agent"

APPFORGE_LOCAL_AGENT_IMAGE="$image" docker compose --env-file "$compose_dir/.env.example" \
  -f "$compose_dir/docker-compose.yml" --profile registration --profile maintenance config --format json |
  python3 -c '
import json, sys
config = json.load(sys.stdin)
agent = config["services"]["local-agent"]
assert not agent.get("ports"), "Local Agent must not publish inbound ports"
assert agent.get("read_only") is True
assert agent.get("user") == "65532:65532"
assert "ALL" in agent.get("cap_drop", [])
assert "no-new-privileges:true" in agent.get("security_opt", [])
assert agent.get("healthcheck", {}).get("test")
'

docker run --rm --entrypoint sh "$image" -c '
test "$(id -u)" = 65532
test "$(id -g)" = 65532
appforge-local-agent version | grep -Eq "protocol=3$"
command -v appforge-local-build >/dev/null
'

secret_volume=$(docker volume create --label appforge.acceptance=local-agent-secret)
cleanup() {
  if [[ $(docker volume inspect -f '{{index .Labels "appforge.acceptance"}}' "$secret_volume" 2>/dev/null || true) == local-agent-secret ]]; then
    docker volume rm "$secret_volume" >/dev/null
  fi
}
trap cleanup EXIT

printf '%s' '{"keystorePassword":"store-fixture","keyPassword":"key-fixture"}' |
  docker run --rm -i -v "$secret_volume:/etc/appforge/local-secrets" "$image" \
    secret-import --name acceptance.json --input-stdin >/dev/null
docker run --rm --entrypoint sh -v "$secret_volume:/etc/appforge/local-secrets:ro" "$image" -c '
test "$(stat -c %a /etc/appforge/local-secrets/acceptance.json)" = 600
grep -q store-fixture /etc/appforge/local-secrets/acceptance.json
'
if printf '%s' '{"keystorePassword":"store","keyPassword":"key","command":"sh"}' |
  docker run --rm -i -v "$secret_volume:/etc/appforge/local-secrets" "$image" \
    secret-import --name rejected.json --input-stdin >/dev/null 2>&1; then
  echo "验收失败: Secret 导入接受了未知命令字段" >&2
  exit 1
fi

APPFORGE_LOCAL_AGENT_IMAGE="$image" "$repo_root/deploy/acceptance/local-agent-executor.sh"
echo "通过: Local Agent 客户交付 Compose、非 root 安全基线、私有 Secret 卷和真实 APK 执行器"
