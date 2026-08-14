#!/usr/bin/env bash

set -euo pipefail

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
env_file=${1:-$delivery_dir/.env}

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
  [[ -r "$delivery_dir/$path" || -r "$path" ]] || { echo "证书文件不可读: $path" >&2; exit 1; }
done

"$delivery_dir/render-config.sh" "$env_file"
docker compose --env-file "$env_file" -f "$delivery_dir/docker-compose.yml" config --quiet
echo "AppForge 生产配置预检通过: mode=$APPFORGE_DEPLOYMENT_MODE version=$APPFORGE_VERSION"
