#!/usr/bin/env bash

set -euo pipefail

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$delivery_dir"

[[ -f .env ]] || { echo "缺少 .env；请复制 .env.example 后填写" >&2; exit 1; }
command -v docker >/dev/null || { echo "未安装 Docker" >&2; exit 1; }
docker compose version >/dev/null

set -a
# shellcheck disable=SC1091
source ./.env
set +a

[[ ${APPFORGE_LOCAL_AGENT_IMAGE:-} == *:* && ${APPFORGE_LOCAL_AGENT_IMAGE##*:} != latest ]] || {
  echo "APPFORGE_LOCAL_AGENT_IMAGE 必须使用非 latest 的固定版本标签" >&2; exit 1;
}
[[ ${APPFORGE_CONTROL_URL:-} == https://* ]] || { echo "控制面地址必须使用 HTTPS" >&2; exit 1; }
[[ ${APPFORGE_LOCAL_AGENT_GATEWAY_URL:-} == https://* ]] || { echo "Agent Gateway 地址必须使用 HTTPS" >&2; exit 1; }
[[ ${APPFORGE_LOCAL_AGENT_MAX_CONCURRENCY:-} =~ ^[0-9]+$ ]] || { echo "并发数必须是整数" >&2; exit 1; }
(( APPFORGE_LOCAL_AGENT_MAX_CONCURRENCY >= 1 && APPFORGE_LOCAL_AGENT_MAX_CONCURRENCY <= 64 )) || {
  echo "并发数必须在 1 到 64 之间" >&2; exit 1;
}
if grep -Eiq 'registration.*token|keystore.*password|key.*password' .env; then
  echo ".env 禁止保存注册码或签名密码" >&2; exit 1
fi

control_ca=${APPFORGE_CONTROL_CA_FILE:-./runtime/control-ca.crt}
gateway_ca=${APPFORGE_GATEWAY_CA_FILE:-./runtime/gateway-ca.crt}
for ca_file in "$control_ca" "$gateway_ca"; do
  [[ -r $ca_file ]] || { echo "CA 文件不可读: $ca_file" >&2; exit 1; }
  grep -q 'BEGIN CERTIFICATE' "$ca_file" || { echo "CA 文件不是 PEM 证书: $ca_file" >&2; exit 1; }
done

docker compose config --quiet
docker run --rm --entrypoint sh "$APPFORGE_LOCAL_AGENT_IMAGE" -c 'test "$(id -u)" = 65532 && test "$(id -g)" = 65532'
docker run --rm "$APPFORGE_LOCAL_AGENT_IMAGE" version | grep -Eq 'protocol=3$'
echo "通过: Local Agent 镜像、HTTPS、CA、非 root 用户、协议版本和 Compose 配置检查"
