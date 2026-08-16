#!/usr/bin/env bash

set -euo pipefail

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
env_file=${1:-$delivery_dir/.env}

resolve_delivery_path() {
  case "$1" in
    /*) printf '%s\n' "$1" ;;
    *) printf '%s/%s\n' "$delivery_dir" "$1" ;;
  esac
}

[[ -f "$env_file" ]] || { echo "缺少生产配置: $env_file" >&2; exit 1; }
set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

required=(APPFORGE_DEPLOYMENT_MODE APPFORGE_VERSION APPFORGE_SCHEMA_VERSION APPFORGE_IMAGE_REGISTRY APPFORGE_PUBLIC_ORIGIN
  APPFORGE_MYSQL_DATABASE APPFORGE_MYSQL_USER APPFORGE_MYSQL_PASSWORD APPFORGE_MYSQL_ROOT_PASSWORD
  APPFORGE_REDIS_PASSWORD APPFORGE_MINIO_ACCESS_KEY APPFORGE_MINIO_SECRET_KEY APPFORGE_INTERNAL_RPC_TOKEN
  APPFORGE_JWT_ACCESS_SECRET APPFORGE_SECRET_MASTER_KEY_BASE64 APPFORGE_TLS_CERT_FILE APPFORGE_TLS_KEY_FILE
  APPFORGE_LOCAL_AGENT_CA_CERT_FILE APPFORGE_LOCAL_AGENT_CA_KEY_FILE APPFORGE_BOOTSTRAP_ADMIN_USERNAME
  APPFORGE_BOOTSTRAP_PASSWORD_BCRYPT_BASE64 APPFORGE_LICENSE_FILE APPFORGE_LICENSE_PUBLIC_KEY_FILE
  APPFORGE_DEPLOYMENT_ID APPFORGE_SIEM_ENDPOINT APPFORGE_SIEM_CA_FILE)

for name in "${required[@]}"; do
  value=${!name:-}
  [[ -n "$value" ]] || { echo "缺少配置: $name" >&2; exit 1; }
  [[ "$value" != *CHANGE_ME* ]] || { echo "配置仍为占位值: $name" >&2; exit 1; }
done

for name in APPFORGE_MYSQL_DATABASE APPFORGE_MYSQL_USER APPFORGE_MYSQL_PASSWORD APPFORGE_REDIS_PASSWORD APPFORGE_MINIO_ACCESS_KEY APPFORGE_MINIO_SECRET_KEY APPFORGE_INTERNAL_RPC_TOKEN APPFORGE_JWT_ACCESS_SECRET APPFORGE_SECRET_MASTER_KEY_BASE64; do
  value=${!name}
  [[ "$value" =~ ^[A-Za-z0-9._~+/=-]+$ ]] || { echo "$name 只能包含适合无歧义配置渲染的安全字符" >&2; exit 1; }
done

case "$APPFORGE_DEPLOYMENT_MODE" in dedicated|private|hybrid) ;; *) echo "无效部署模式" >&2; exit 1;; esac
[[ "$APPFORGE_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]] || { echo "APPFORGE_VERSION 必须为语义化版本" >&2; exit 1; }
version_core=${APPFORGE_VERSION%%[-+]*}
version_line=${version_core%.*}
case "$version_line" in
  1.0) expected_schema=20260814_110_v4_builder_node_recovery ;;
  1.1) expected_schema=20260815_112_v7_customer_storage ;;
  1.2) expected_schema=20260815_113_v7_air_gapped ;;
  *) echo "APPFORGE_VERSION 不在已发布兼容矩阵中: $APPFORGE_VERSION" >&2; exit 1 ;;
esac
[[ $APPFORGE_SCHEMA_VERSION == "$expected_schema" ]] || {
  echo "版本与Schema不兼容: version=$APPFORGE_VERSION schema=$APPFORGE_SCHEMA_VERSION expected=$expected_schema" >&2
  exit 1
}
[[ "$APPFORGE_PUBLIC_ORIGIN" =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || { echo "生产入口必须是无路径和查询参数的 https Origin" >&2; exit 1; }
if [[ $APPFORGE_SIEM_ENDPOINT == https://* ]]; then
  [[ "$APPFORGE_SIEM_ENDPOINT" =~ ^https://[A-Za-z0-9._~:/?=+-]+$ ]] || { echo "SIEM HTTPS Endpoint包含不安全字符" >&2; exit 1; }
  [[ -n ${APPFORGE_SIEM_TOKEN_FILE:-} && ${APPFORGE_SIEM_TOKEN_FILE:-} != *CHANGE_ME* ]] || {
    echo "SIEM HTTPS Webhook必须配置Bearer Token文件" >&2
    exit 1
  }
elif [[ $APPFORGE_SIEM_ENDPOINT == syslog+tls://* ]]; then
  [[ "$APPFORGE_SIEM_ENDPOINT" =~ ^syslog\+tls://([A-Za-z0-9.-]+|\[[0-9A-Fa-f:]+\]):[0-9]{1,5}$ ]] || {
    echo "SIEM Syslog Endpoint必须是无凭据、路径和查询的syslog+tls://host:port" >&2
    exit 1
  }
  siem_port=${APPFORGE_SIEM_ENDPOINT##*:}
  (( siem_port >= 1 && siem_port <= 65535 )) || { echo "SIEM Syslog端口无效" >&2; exit 1; }
else
  echo "SIEM Endpoint必须使用https://或syslog+tls://" >&2
  exit 1
fi
[[ ${APPFORGE_PROMETHEUS_ENABLED:-true} == true || ${APPFORGE_PROMETHEUS_ENABLED:-true} == false ]] || { echo "APPFORGE_PROMETHEUS_ENABLED 只能为 true 或 false" >&2; exit 1; }
prometheus_port=${APPFORGE_PROMETHEUS_PORT:-9101}
[[ $prometheus_port =~ ^[0-9]+$ ]] && (( prometheus_port >= 1 && prometheus_port <= 65535 )) || { echo "APPFORGE_PROMETHEUS_PORT 必须为有效端口" >&2; exit 1; }
[[ ${APPFORGE_PROMETHEUS_PATH:-/metrics} =~ ^/[A-Za-z0-9._~/-]+$ ]] || { echo "APPFORGE_PROMETHEUS_PATH 必须为无查询参数的绝对路径" >&2; exit 1; }
if [[ -n ${APPFORGE_OTLP_ENDPOINT:-} ]]; then
  [[ $APPFORGE_OTLP_ENDPOINT =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?(/[A-Za-z0-9._~/-]*)?$ ]] || { echo "APPFORGE_OTLP_ENDPOINT 必须为无凭据、查询和片段的 HTTPS URL" >&2; exit 1; }
fi
[[ ${APPFORGE_OTLP_SAMPLER:-0.1} =~ ^(0(\.[0-9]+)?|1(\.0+)?)$ ]] || { echo "APPFORGE_OTLP_SAMPLER 必须在 0 到 1 之间" >&2; exit 1; }

egress_enabled=${APPFORGE_EGRESS_PROXY_ENABLED:-false}
[[ $egress_enabled == true || $egress_enabled == false ]] || { echo "APPFORGE_EGRESS_PROXY_ENABLED 只能为 true 或 false" >&2; exit 1; }
egress_max_connections=${APPFORGE_EGRESS_PROXY_MAX_CONNECTIONS:-256}
[[ $egress_max_connections =~ ^[0-9]+$ ]] &&
  (( egress_max_connections >= 1 && egress_max_connections <= 10000 )) || {
  echo "APPFORGE_EGRESS_PROXY_MAX_CONNECTIONS 必须在 1 到 10000 之间" >&2
  exit 1
}
if [[ $egress_enabled == true ]]; then
  [[ ${APPFORGE_EGRESS_PROXY_URL:-} == http://egress-proxy:3128 ]] || {
    echo "内置出口代理启用时 APPFORGE_EGRESS_PROXY_URL 必须为 http://egress-proxy:3128" >&2
    exit 1
  }
  allowlist_config=${APPFORGE_EGRESS_PROXY_ALLOWLIST_FILE:-}
  [[ -n $allowlist_config ]] || { echo "启用出口代理必须配置 Allowlist 文件" >&2; exit 1; }
  allowlist_path=$(resolve_delivery_path "$allowlist_config")
  [[ -f $allowlist_path && ! -L $allowlist_path ]] || { echo "出口 Allowlist 必须是普通文件且不能是符号链接" >&2; exit 1; }
  allowlist_mode=$(stat -c %a "$allowlist_path" 2>/dev/null || stat -f %Lp "$allowlist_path")
  (( (8#$allowlist_mode & 8#022) == 0 )) || { echo "出口 Allowlist 不能被 group/others 修改" >&2; exit 1; }
  allowlist_count=0
  while IFS= read -r raw_line || [[ -n $raw_line ]]; do
    line=${raw_line%%#*}
    line=${line#"${line%%[![:space:]]*}"}
    line=${line%"${line##*[![:space:]]}"}
    [[ -n $line ]] || continue
    ((allowlist_count += 1))
    [[ $line != *example.com* ]] || { echo "出口 Allowlist 仍包含 example.com 占位域名" >&2; exit 1; }
    [[ $line =~ ^(\*\.)?([A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?\.)*[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?:[0-9]{1,5}$ ||
       $line =~ ^\[[0-9A-Fa-f:]+\]:[0-9]{1,5}$ ]] || {
      echo "出口 Allowlist 条目无效: $line" >&2
      exit 1
    }
    port=${line##*:}
    (( port >= 1 && port <= 65535 )) || { echo "出口 Allowlist 端口无效: $line" >&2; exit 1; }
  done <"$allowlist_path"
  (( allowlist_count >= 1 && allowlist_count <= 1024 )) || { echo "出口 Allowlist 必须包含 1 到 1024 个目标" >&2; exit 1; }
  for internal_host in localhost 127.0.0.1 mysql redis etcd minio system-rpc core-rpc builder-rpc; do
    case ",${APPFORGE_EGRESS_NO_PROXY:-}," in
      *",$internal_host,"*) ;;
      *) echo "APPFORGE_EGRESS_NO_PROXY 缺少内部目标: $internal_host" >&2; exit 1 ;;
    esac
  done
elif [[ -n ${APPFORGE_EGRESS_PROXY_URL:-} ]]; then
  echo "出口代理未启用时 APPFORGE_EGRESS_PROXY_URL 必须留空" >&2
  exit 1
fi
for name in APPFORGE_STRIPE_SECRET_KEY APPFORGE_STRIPE_WEBHOOK_SECRET; do
  value=${!name:-}
  [[ "$value" =~ ^[A-Za-z0-9._~+/=-]*$ ]] || { echo "$name 包含不安全的配置渲染字符" >&2; exit 1; }
done

if [[ -n ${APPFORGE_REPLICA_ENDPOINT:-} ]]; then
  [[ $APPFORGE_REPLICA_ENDPOINT =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || {
    echo "生产对象副本 Endpoint 必须是无路径和查询参数的 HTTPS Origin" >&2
    exit 1
  }
  for name in APPFORGE_REPLICA_ACCESS_KEY APPFORGE_REPLICA_SECRET_KEY; do
    value=${!name:-}
    [[ -n $value && $value != *CHANGE_ME* && $value =~ ^[A-Za-z0-9._~+/=-]+$ ]] || {
      echo "$name 缺失、仍为占位值或包含不安全字符" >&2
      exit 1
    }
  done
  [[ ${#APPFORGE_REPLICA_SECRET_KEY} -ge 24 ]] || { echo "APPFORGE_REPLICA_SECRET_KEY 至少需要 24 个字符" >&2; exit 1; }
  [[ ${APPFORGE_REPLICA_BUCKET:-} =~ ^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$ ]] || { echo "APPFORGE_REPLICA_BUCKET 不是有效桶名" >&2; exit 1; }
  [[ ${APPFORGE_REPLICA_RULE_ID:-} =~ ^[A-Za-z0-9._-]{1,64}$ ]] || { echo "APPFORGE_REPLICA_RULE_ID 无效" >&2; exit 1; }
  [[ ${APPFORGE_REPLICA_SYNC:-false} == false || ${APPFORGE_REPLICA_SYNC:-false} == true ]] || { echo "APPFORGE_REPLICA_SYNC 只能为 true 或 false" >&2; exit 1; }
fi

for name in APPFORGE_MYSQL_PASSWORD APPFORGE_MYSQL_ROOT_PASSWORD APPFORGE_REDIS_PASSWORD APPFORGE_MINIO_SECRET_KEY APPFORGE_INTERNAL_RPC_TOKEN APPFORGE_JWT_ACCESS_SECRET; do
  value=${!name}
  [[ ${#value} -ge 24 ]] || { echo "$name 至少需要 24 个字符" >&2; exit 1; }
done

readable_paths=("$APPFORGE_TLS_CERT_FILE" "$APPFORGE_TLS_KEY_FILE" "$APPFORGE_LOCAL_AGENT_CA_CERT_FILE" "$APPFORGE_LOCAL_AGENT_CA_KEY_FILE" "$APPFORGE_LICENSE_FILE" "$APPFORGE_LICENSE_PUBLIC_KEY_FILE" "$APPFORGE_SIEM_CA_FILE")
[[ -z ${APPFORGE_SIEM_TOKEN_FILE:-} ]] || readable_paths+=("$APPFORGE_SIEM_TOKEN_FILE")
for path in "${readable_paths[@]}"; do
  [[ -r $(resolve_delivery_path "$path") ]] || { echo "证书文件不可读: $path" >&2; exit 1; }
done

private_paths=("$env_file" "$APPFORGE_TLS_KEY_FILE" "$APPFORGE_LOCAL_AGENT_CA_KEY_FILE")
[[ -z ${APPFORGE_SIEM_TOKEN_FILE:-} ]] || private_paths+=("$APPFORGE_SIEM_TOKEN_FILE")
[[ -z ${APPFORGE_VAULT_TOKEN_FILE:-} ]] || private_paths+=("$APPFORGE_VAULT_TOKEN_FILE")
for configured_path in "${private_paths[@]}"; do
  private_path=$configured_path
  [[ -e $private_path ]] || private_path="$delivery_dir/$configured_path"
  [[ -f $private_path && ! -L $private_path ]] || { echo "私有配置必须是普通文件且不能是符号链接: $configured_path" >&2; exit 1; }
  private_mode=$(stat -c %a "$private_path" 2>/dev/null || stat -f %Lp "$private_path")
  [[ $private_mode =~ ^[0-7]{3,4}$ ]] || { echo "无法识别私有文件权限: $configured_path" >&2; exit 1; }
  (( (8#$private_mode & 8#077) == 0 )) || { echo "私有文件不能向group/others开放: $configured_path ($private_mode)" >&2; exit 1; }
done

command -v openssl >/dev/null || { echo "生产预检需要openssl" >&2; exit 1; }
tls_cert=$(resolve_delivery_path "$APPFORGE_TLS_CERT_FILE")
tls_key=$(resolve_delivery_path "$APPFORGE_TLS_KEY_FILE")
agent_ca_cert=$(resolve_delivery_path "$APPFORGE_LOCAL_AGENT_CA_CERT_FILE")
agent_ca_key=$(resolve_delivery_path "$APPFORGE_LOCAL_AGENT_CA_KEY_FILE")
for cert in "$tls_cert" "$agent_ca_cert"; do
  openssl x509 -in "$cert" -checkend 3600 -noout >/dev/null 2>&1 || {
    echo "证书格式无效或将在1小时内过期: $cert" >&2
    exit 1
  }
done
certificate_public_key() {
  openssl x509 -in "$1" -pubkey -noout 2>/dev/null | openssl pkey -pubin -outform DER 2>/dev/null | openssl dgst -sha256
}
private_public_key() {
  openssl pkey -in "$1" -pubout -outform DER 2>/dev/null | openssl dgst -sha256
}
[[ $(certificate_public_key "$tls_cert") == "$(private_public_key "$tls_key")" ]] || {
  echo "TLS证书与私钥不匹配" >&2
  exit 1
}
openssl x509 -in "$agent_ca_cert" -noout -text | grep -q 'Public Key Algorithm: id-ecPublicKey' || {
  echo "Local Agent CA必须使用ECDSA证书" >&2
  exit 1
}
[[ $(certificate_public_key "$agent_ca_cert") == "$(private_public_key "$agent_ca_key")" ]] || {
  echo "Local Agent CA证书与私钥不匹配" >&2
  exit 1
}
openssl x509 -in "$agent_ca_cert" -noout -text | grep -q 'CA:TRUE' || {
  echo "Local Agent CA证书缺少CA:TRUE约束" >&2
  exit 1
}

"$delivery_dir/render-config.sh" "$env_file"
compose_profiles=()
[[ $egress_enabled == false ]] || compose_profiles=(--profile egress)
docker compose --env-file "$env_file" -f "$delivery_dir/docker-compose.yml" "${compose_profiles[@]}" config --quiet
echo "AppForge 生产配置预检通过: mode=$APPFORGE_DEPLOYMENT_MODE version=$APPFORGE_VERSION schema=$APPFORGE_SCHEMA_VERSION"
