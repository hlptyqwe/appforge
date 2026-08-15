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

required=(APPFORGE_DEPLOYMENT_MODE APPFORGE_VERSION APPFORGE_IMAGE_REGISTRY APPFORGE_PUBLIC_ORIGIN
  APPFORGE_MYSQL_DATABASE APPFORGE_MYSQL_USER APPFORGE_MYSQL_PASSWORD APPFORGE_MYSQL_ROOT_PASSWORD
  APPFORGE_REDIS_PASSWORD APPFORGE_MINIO_ACCESS_KEY APPFORGE_MINIO_SECRET_KEY APPFORGE_INTERNAL_RPC_TOKEN
  APPFORGE_JWT_ACCESS_SECRET APPFORGE_SECRET_MASTER_KEY_BASE64 APPFORGE_TLS_CERT_FILE APPFORGE_TLS_KEY_FILE
  APPFORGE_LOCAL_AGENT_CA_CERT_FILE APPFORGE_LOCAL_AGENT_CA_KEY_FILE APPFORGE_BOOTSTRAP_ADMIN_USERNAME
  APPFORGE_BOOTSTRAP_PASSWORD_BCRYPT_BASE64 APPFORGE_LICENSE_FILE APPFORGE_LICENSE_PUBLIC_KEY_FILE
  APPFORGE_DEPLOYMENT_ID APPFORGE_SIEM_ENDPOINT APPFORGE_SIEM_TOKEN_FILE APPFORGE_SIEM_CA_FILE)

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
[[ "$APPFORGE_PUBLIC_ORIGIN" =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || { echo "生产入口必须是无路径和查询参数的 https Origin" >&2; exit 1; }
[[ "$APPFORGE_SIEM_ENDPOINT" =~ ^https://[A-Za-z0-9._~:/?=+-]+$ ]] || { echo "SIEM Endpoint必须是安全字符组成的HTTPS URL" >&2; exit 1; }
for name in APPFORGE_STRIPE_SECRET_KEY APPFORGE_STRIPE_WEBHOOK_SECRET; do
  value=${!name:-}
  [[ "$value" =~ ^[A-Za-z0-9._~+/=-]*$ ]] || { echo "$name 包含不安全的配置渲染字符" >&2; exit 1; }
done

for name in APPFORGE_MYSQL_PASSWORD APPFORGE_MYSQL_ROOT_PASSWORD APPFORGE_REDIS_PASSWORD APPFORGE_MINIO_SECRET_KEY APPFORGE_INTERNAL_RPC_TOKEN APPFORGE_JWT_ACCESS_SECRET; do
  value=${!name}
  [[ ${#value} -ge 24 ]] || { echo "$name 至少需要 24 个字符" >&2; exit 1; }
done

for path in "$APPFORGE_TLS_CERT_FILE" "$APPFORGE_TLS_KEY_FILE" "$APPFORGE_LOCAL_AGENT_CA_CERT_FILE" "$APPFORGE_LOCAL_AGENT_CA_KEY_FILE" "$APPFORGE_LICENSE_FILE" "$APPFORGE_LICENSE_PUBLIC_KEY_FILE" "$APPFORGE_SIEM_TOKEN_FILE" "$APPFORGE_SIEM_CA_FILE"; do
  [[ -r $(resolve_delivery_path "$path") ]] || { echo "证书文件不可读: $path" >&2; exit 1; }
done

private_paths=("$env_file" "$APPFORGE_TLS_KEY_FILE" "$APPFORGE_LOCAL_AGENT_CA_KEY_FILE" "$APPFORGE_SIEM_TOKEN_FILE")
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
docker compose --env-file "$env_file" -f "$delivery_dir/docker-compose.yml" config --quiet
echo "AppForge 生产配置预检通过: mode=$APPFORGE_DEPLOYMENT_MODE version=$APPFORGE_VERSION"
