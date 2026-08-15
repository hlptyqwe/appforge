#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 1 ]] || { echo "用法: $0 OUTPUT.tar.gz" >&2; exit 1; }
output=$1
[[ ! -e $output ]] || { echo "诊断包已存在，拒绝覆盖: $output" >&2; exit 1; }

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
env_file=${APPFORGE_ENV_FILE:-$delivery_dir/.env}
[[ -r $env_file ]] || { echo "环境文件不可读: $env_file" >&2; exit 1; }
set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

compose_file=${APPFORGE_COMPOSE_FILE:-$delivery_dir/docker-compose.yml}
api_url=${APPFORGE_DIAGNOSTIC_API_URL:-${APPFORGE_PUBLIC_ORIGIN:?required}}
staging=$(mktemp -d)
cleanup() { rm -rf "$staging"; }
trap cleanup EXIT
umask 077

APPFORGE_DIAG_VERSION=${APPFORGE_VERSION:-unknown} \
APPFORGE_DIAG_MODE=${APPFORGE_DEPLOYMENT_MODE:-unknown} \
APPFORGE_DIAG_ORIGIN=${APPFORGE_PUBLIC_ORIGIN:-unknown} \
python3 -c '
import datetime, json, os
print(json.dumps({
    "generatedAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "version": os.environ["APPFORGE_DIAG_VERSION"],
    "deploymentMode": os.environ["APPFORGE_DIAG_MODE"],
    "publicOrigin": os.environ["APPFORGE_DIAG_ORIGIN"],
}, ensure_ascii=False))
' >"$staging/metadata.json"

printf 'service\tcontainer\timage\tstate\thealth\n' >"$staging/components.tsv"
docker compose -f "$compose_file" ps --services --all | LC_ALL=C sort | while IFS= read -r service; do
  container_id=$(docker compose -f "$compose_file" ps -q --all "$service")
  [[ -n $container_id ]] || { printf '%s\t-\t-\tnot-created\t-\n' "$service"; continue; }
  docker inspect -f '{{index .Config.Labels "com.docker.compose.service"}}\t{{.Name}}\t{{.Config.Image}}\t{{.State.Status}}\t{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container_id" |
    sed 's#\t/#\t#'
done >>"$staging/components.tsv"

for endpoint in healthz readyz; do
  if ! curl -fsS --max-time 10 --max-filesize 65536 "${api_url%/}/$endpoint" >"$staging/$endpoint.json"; then
    printf '{"status":"unreachable"}\n' >"$staging/$endpoint.json"
  fi
done

docker compose -f "$compose_file" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" mysql -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -N -B -e "SELECT version,applied_at FROM sys_schema_migration ORDER BY applied_at,version"' \
  >"$staging/schema-migrations.tsv"
docker compose -f "$compose_file" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" mysql -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -N -B -e "SELECT status,drain_status,protocol_version,COUNT(*) FROM t_local_agent GROUP BY status,drain_status,protocol_version ORDER BY status,drain_status,protocol_version"' \
  >"$staging/local-agent-summary.tsv"

{
  for file in "$compose_file" "$delivery_dir"/*.sh; do
    [[ -f $file ]] && sha256sum "$file"
  done
} >"$staging/deployment-file-sha256.tsv"

for sensitive_name in APPFORGE_MYSQL_PASSWORD APPFORGE_MYSQL_ROOT_PASSWORD APPFORGE_REDIS_PASSWORD \
  APPFORGE_MINIO_SECRET_KEY APPFORGE_INTERNAL_RPC_TOKEN APPFORGE_JWT_ACCESS_SECRET \
  APPFORGE_SECRET_MASTER_KEY_BASE64 APPFORGE_STRIPE_SECRET_KEY APPFORGE_STRIPE_WEBHOOK_SECRET; do
  sensitive_value=${!sensitive_name:-}
  if [[ -n $sensitive_value && $sensitive_value != CHANGE_ME* ]] && grep -R -F -q -- "$sensitive_value" "$staging"; then
    echo "诊断包敏感值扫描失败: $sensitive_name" >&2
    exit 1
  fi
done
if grep -R -E -q -- 'BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|keystorePassword|keyPassword' "$staging"; then
  echo "诊断包包含禁止的密钥或密码字段" >&2
  exit 1
fi

manifest="$staging/SHA256SUMS"
(cd "$staging" && sha256sum metadata.json components.tsv healthz.json readyz.json schema-migrations.tsv \
  local-agent-summary.tsv deployment-file-sha256.tsv >"$manifest")
tar -C "$staging" -czf "$output" .
chmod 0600 "$output"
echo "脱敏诊断包已生成: $output"
