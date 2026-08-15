#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
fixtures="$repo_root/deploy/acceptance/fixtures"
image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
go_cache=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache}
runtime_root=$(mktemp -d /tmp/appforge-v7-local-agent.XXXXXX)
project="appforge-v7-agent-${RANDOM}-$$"
gateway_pid=
good_image="appforge-local-agent:v7-recovery-${RANDOM}-$$"
bad_image="appforge-local-agent:v7-unhealthy-${RANDOM}-$$"

export COMPOSE_PROJECT_NAME=$project
export COMPOSE_FILE=docker-compose.yml:acceptance.compose.yml
export APPFORGE_ACCEPTANCE_EXECUTOR="$fixtures/local-agent-runtime-executor.sh"
export APPFORGE_ACCEPTANCE_ROTATE_BEFORE=48h

cleanup() {
  set +e
  if [[ -n $gateway_pid ]]; then
    kill "$gateway_pid" >/dev/null 2>&1 || true
    wait "$gateway_pid" >/dev/null 2>&1 || true
  fi
  if [[ -d $runtime_root/delivery ]]; then
    (cd "$runtime_root/delivery" && docker compose down -v --remove-orphans >/dev/null 2>&1) || true
  fi
  docker image rm "$bad_image" >/dev/null 2>&1 || true
  docker image rm "$good_image" >/dev/null 2>&1 || true
  rm -rf "$runtime_root"
}
trap cleanup EXIT

for command_name in docker openssl curl go python3; do
  command -v "$command_name" >/dev/null || { echo "验收失败: 缺少命令 $command_name" >&2; exit 1; }
done
docker image inspect "$image" >/dev/null 2>&1 || { echo "验收失败: Local Agent 镜像不存在: $image" >&2; exit 1; }

mkdir -p "$runtime_root/certs" "$runtime_root/delivery/runtime"
cp -R "$repo_root/deploy/local-agent/." "$runtime_root/delivery/"
cp "$fixtures/local-agent-runtime.compose.yml" "$runtime_root/delivery/acceptance.compose.yml"
chmod 0755 "$runtime_root/delivery/"*.sh

openssl ecparam -name prime256v1 -genkey -noout -out "$runtime_root/certs/ca.key"
openssl req -x509 -new -sha256 -key "$runtime_root/certs/ca.key" -days 30 \
  -subj "/CN=AppForge V7 Local Agent Acceptance CA" -out "$runtime_root/certs/ca.crt"
openssl ecparam -name prime256v1 -genkey -noout -out "$runtime_root/certs/server.key"
openssl req -new -sha256 -key "$runtime_root/certs/server.key" \
  -subj "/CN=host.docker.internal" -out "$runtime_root/certs/server.csr"
printf '%s\n' 'basicConstraints=critical,CA:FALSE' 'keyUsage=critical,digitalSignature,keyEncipherment' \
  'extendedKeyUsage=serverAuth' 'subjectAltName=DNS:host.docker.internal,DNS:localhost,IP:127.0.0.1' \
  >"$runtime_root/certs/server.ext"
openssl x509 -req -sha256 -in "$runtime_root/certs/server.csr" -CA "$runtime_root/certs/ca.crt" \
  -CAkey "$runtime_root/certs/ca.key" -CAcreateserial -days 30 -extfile "$runtime_root/certs/server.ext" \
  -out "$runtime_root/certs/server.crt" >/dev/null 2>&1
openssl ecparam -name prime256v1 -genkey -noout -out "$runtime_root/certs/client.key"
openssl req -new -sha256 -key "$runtime_root/certs/client.key" \
  -subj "/CN=local-agent-81" -out "$runtime_root/certs/client.csr"
printf '%s\n' 'basicConstraints=critical,CA:FALSE' 'keyUsage=critical,digitalSignature' \
  'extendedKeyUsage=clientAuth' >"$runtime_root/certs/client.ext"
openssl x509 -req -sha256 -in "$runtime_root/certs/client.csr" -CA "$runtime_root/certs/ca.crt" \
  -CAkey "$runtime_root/certs/ca.key" -CAcreateserial -days 1 -extfile "$runtime_root/certs/client.ext" \
  -out "$runtime_root/certs/client.crt" >/dev/null 2>&1
chmod 0600 "$runtime_root/certs/"*.key "$runtime_root/certs/"*.crt

port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')
GOCACHE="$go_cache" go build -o "$runtime_root/local-agent-gateway" "$fixtures/local-agent-gateway.go"
"$runtime_root/local-agent-gateway" --addr "0.0.0.0:$port" \
  --cert "$runtime_root/certs/server.crt" --key "$runtime_root/certs/server.key" \
  --client-ca "$runtime_root/certs/ca.crt" --signing-ca "$runtime_root/certs/ca.crt" \
  --signing-key "$runtime_root/certs/ca.key" --events "$runtime_root/events.jsonl" \
  --revoke-file "$runtime_root/revoked" \
  >"$runtime_root/gateway.log" 2>&1 &
gateway_pid=$!
for _ in $(seq 1 50); do
  if curl -fsS --cacert "$runtime_root/certs/ca.crt" --cert "$runtime_root/certs/client.crt" \
    --key "$runtime_root/certs/client.key" "https://localhost:$port/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
curl -fsS --cacert "$runtime_root/certs/ca.crt" --cert "$runtime_root/certs/client.crt" \
  --key "$runtime_root/certs/client.key" "https://localhost:$port/health" >/dev/null

cp "$runtime_root/certs/ca.crt" "$runtime_root/delivery/runtime/control-ca.crt"
cp "$runtime_root/certs/ca.crt" "$runtime_root/delivery/runtime/gateway-ca.crt"
printf '%s\n' \
  "APPFORGE_LOCAL_AGENT_IMAGE=$image" \
  "APPFORGE_CONTROL_URL=https://localhost:$port" \
  "APPFORGE_LOCAL_AGENT_GATEWAY_URL=https://host.docker.internal:$port" \
  'APPFORGE_LOCAL_AGENT_MAX_CONCURRENCY=1' \
  'APPFORGE_LOCAL_AGENT_LEASE_SECONDS=6' \
  'APPFORGE_LOCAL_AGENT_ROTATE_BEFORE=48h' >"$runtime_root/delivery/.env"
chmod 0600 "$runtime_root/delivery/.env"

cd "$runtime_root/delivery"
docker compose create local-agent >/dev/null
state_volume="${project}_agent-state"
secret_volume="${project}_agent-secrets"
acceptance_volume="${project}_acceptance-state"
docker run --rm --user 0:0 --entrypoint sh -v "$acceptance_volume:/acceptance" "$image" -c \
  'chown 65532:65532 /acceptance; chmod 0700 /acceptance'

printf '{"agentId":81,"gatewayUrl":"https://host.docker.internal:%s","certificate":"/var/lib/appforge-agent/client.crt","privateKey":"/var/lib/appforge-agent/client.key","clientCa":"/var/lib/appforge-agent/ca.crt","gatewayCa":"/var/lib/appforge-agent/ca.crt","protocol":3,"agentVersion":"1.1.0","lastTimestamp":1}' \
  "$port" >"$runtime_root/certs/state.json"
chmod 0600 "$runtime_root/certs/state.json"
docker run --rm --entrypoint sh -v "$state_volume:/state" -v "$runtime_root/certs:/fixture:ro" "$image" -c \
  'cp /fixture/client.crt /state/client.crt; cp /fixture/client.key /state/client.key; cp /fixture/ca.crt /state/ca.crt; cp /fixture/state.json /state/state.json; chmod 0600 /state/*'
printf '%s' '{"keystorePassword":"changeit","keyPassword":"changeit"}' |
  docker run --rm -i -v "$secret_volume:/etc/appforge/local-secrets" "$image" \
    secret-import --name acceptance.json --input-stdin >/dev/null

marker_exists() {
  docker run --rm --entrypoint sh -v "$acceptance_volume:/acceptance:ro" "$image" -c "test -f /acceptance/$1" >/dev/null 2>&1
}
wait_for_marker() {
  local marker=$1 attempts=${2:-120}
  for _ in $(seq 1 "$attempts"); do
    marker_exists "$marker" && return 0
    sleep 1
  done
  echo "验收失败: 等待 Local Agent 标记超时: $marker" >&2
  docker compose logs --tail 120 local-agent >&2 || true
  return 1
}
wait_for_event() {
  local pattern=$1 attempts=${2:-120}
  for _ in $(seq 1 "$attempts"); do
    grep -q "$pattern" "$runtime_root/events.jsonl" 2>/dev/null && return 0
    sleep 1
  done
  echo "验收失败: 等待 Gateway 事件超时: $pattern" >&2
  docker compose logs --tail 120 local-agent >&2 || true
  return 1
}

docker compose up -d local-agent >/dev/null
wait_for_event 'certificate_rotated' 60
rotated_certificate=$(docker run --rm --entrypoint sh -v "$state_volume:/state:ro" "$image" -c \
  'sed -n "s/.*\"certificate\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" /state/state.json')
[[ $rotated_certificate == /var/lib/appforge-agent/client-*.crt ]] || {
  echo "验收失败: Agent 自动轮换后未原子切换证书路径: $rotated_certificate" >&2
  exit 1
}
docker run --rm --entrypoint sh -v "$state_volume:/state:ro" "$image" -c \
  'certificate=$(sed -n "s/.*\"certificate\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" /state/state.json); test -s "/state/${certificate##*/}"'
echo "通过: 真实 Local Agent 检测临近到期证书并通过 mTLS Gateway 自动轮换到新私钥和证书"
wait_for_marker attempt-1.started 90
first_container=$(docker compose ps -q local-agent)
first_timestamp=$(docker run --rm --entrypoint sh -v "$state_volume:/state:ro" "$image" -c 'sed -n "s/.*\"lastTimestamp\"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p" /state/state.json')
docker kill --signal KILL "$first_container" >/dev/null
docker compose up -d --force-recreate local-agent >/dev/null
wait_for_marker attempt-2.completed 150

second_container=$(docker compose ps -q local-agent)
[[ -n $second_container && $second_container != "$first_container" ]] || {
  echo "验收失败: Local Agent 进程中断后没有创建新容器" >&2
  exit 1
}
second_timestamp=$(docker run --rm --entrypoint sh -v "$state_volume:/state:ro" "$image" -c 'sed -n "s/.*\"lastTimestamp\"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p" /state/state.json')
[[ $first_timestamp =~ ^[0-9]+$ && $second_timestamp =~ ^[0-9]+$ && $second_timestamp -gt $first_timestamp ]] || {
  echo "验收失败: Agent 状态卷中的防重放时间戳未跨进程单调增长" >&2
  exit 1
}
docker run --rm --entrypoint sh -v "$acceptance_volume:/acceptance:ro" "$image" -c \
  'apksigner verify --verbose --print-certs /acceptance/channel-attempt-2.apk >/dev/null; aapt dump badging /acceptance/channel-attempt-2.apk | grep -q "package: name='\''com.example.local'\''"'
python3 - "$runtime_root/events.jsonl" <<'PY'
import json, sys
events = [json.loads(line) for line in open(sys.argv[1], encoding="utf-8") if line.strip()]
assert sum(e.get("path") == "attempt_claimed" and e.get("builder_attempt") == 1 for e in events) == 1
assert sum(e.get("path") == "attempt_claimed" and e.get("builder_attempt") == 2 for e in events) == 1
assert sum(e.get("path") == "attempt_completed" and e.get("builder_attempt") == 2 for e in events) == 1
assert not any(e.get("path") == "attempt_completed" and e.get("builder_attempt") == 1 for e in events)
PY

stale_timestamp=$(python3 -c 'import time; print(int(time.time() * 1000))')
stale_status=$(curl -sS -o "$runtime_root/stale-response.json" -w '%{http_code}' \
  --cacert "$runtime_root/certs/ca.crt" --cert "$runtime_root/certs/client.crt" --key "$runtime_root/certs/client.key" \
  -H 'Content-Type: application/json' -d "{\"auth\":{\"agent_id\":81,\"nonce\":\"stale-attempt-$RANDOM\",\"timestamp\":$stale_timestamp},\"task_id\":7001,\"builder_attempt\":1,\"apk_reference\":\"local-agent://stale\",\"apk_sha256\":\"deadbeef\"}" \
  "https://localhost:$port/v1/tasks/complete")
[[ $stale_status == 409 ]] || { echo "验收失败: 旧 attempt 完成请求未被拒绝，HTTP $stale_status" >&2; exit 1; }
echo "通过: Local Agent 在真实 APK 执行中被强制中断后，以新 attempt 恢复，状态卷时间戳持续且旧 attempt 被拒绝"

docker image tag "$image" "$good_image"
APPFORGE_LOCAL_AGENT_UPGRADE_HEALTH_ATTEMPTS=15 APPFORGE_LOCAL_AGENT_UPGRADE_HEALTH_INTERVAL_SECONDS=1 \
  "$runtime_root/delivery/upgrade.sh" --drained --offline "$good_image" >/dev/null
current_container=$(docker compose ps -q local-agent)
[[ $(docker inspect -f '{{.Config.Image}}' "$current_container") == "$good_image" ]] || {
  echo "验收失败: 离线升级后未运行目标镜像" >&2
  exit 1
}
grep -Fxq "APPFORGE_LOCAL_AGENT_IMAGE=$good_image" .env

docker build --build-arg "BASE_IMAGE=$good_image" -t "$bad_image" \
  -f "$fixtures/local-agent-unhealthy.Dockerfile" "$fixtures" >/dev/null
if APPFORGE_LOCAL_AGENT_UPGRADE_HEALTH_ATTEMPTS=8 APPFORGE_LOCAL_AGENT_UPGRADE_HEALTH_INTERVAL_SECONDS=1 \
  "$runtime_root/delivery/upgrade.sh" --drained --offline "$bad_image" >/dev/null 2>&1; then
  echo "验收失败: 不健康 Local Agent 镜像升级意外成功" >&2
  exit 1
fi
rollback_container=$(docker compose ps -q local-agent)
[[ $(docker inspect -f '{{.Config.Image}}' "$rollback_container") == "$good_image" ]] || {
  echo "验收失败: 不健康镜像未自动恢复到升级前镜像" >&2
  exit 1
}
[[ $(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$rollback_container") == healthy ]] || {
  echo "验收失败: 自动恢复后的 Local Agent 不健康" >&2
  exit 1
}
grep -Fxq "APPFORGE_LOCAL_AGENT_IMAGE=$good_image" .env
echo "通过: Local Agent 已导入镜像离线升级、状态卷保留、不健康版本自动回滚及旧版本健康复核"

touch "$runtime_root/revoked"
wait_for_event 'revoked_request_rejected' 30
python3 - "$runtime_root/events.jsonl" <<'PY'
import json, sys
events = [json.loads(line) for line in open(sys.argv[1], encoding="utf-8") if line.strip()]
rotated = [e for e in events if e.get("path") == "certificate_rotated"]
rejected = [e for e in events if e.get("path") == "revoked_request_rejected"]
assert len(rotated) == 1, rotated
assert rejected and rejected[-1].get("certificate_serial") == rotated[0].get("certificate_serial"), (rotated, rejected)
PY
echo "通过: Gateway 吊销开关立即拒绝真实 Agent 当前轮换证书，拒绝事件保留证书序列号"
