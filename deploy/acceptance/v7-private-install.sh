#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
registry=${APPFORGE_PRIVATE_ACCEPTANCE_REGISTRY:-appforge-private-acceptance}
version=${APPFORGE_PRIVATE_ACCEPTANCE_VERSION:-1.1.0}
deployment_mode=${APPFORGE_PRIVATE_ACCEPTANCE_MODE:-private}
case "$deployment_mode" in private|dedicated) ;; *) echo "安装验收模式只能是private或dedicated" >&2; exit 2;; esac
deployment_id="${deployment_mode}-install-acceptance"
project="appforge-v7-${deployment_mode}-${RANDOM}-$$"
temporary=$(mktemp -d)
delivery="$temporary/deploy/production"
admin_port=${APPFORGE_PRIVATE_ACCEPTANCE_ADMIN_PORT:-18443}
agent_port=${APPFORGE_PRIVATE_ACCEPTANCE_AGENT_PORT:-19443}
gateway_port=${APPFORGE_PRIVATE_ACCEPTANCE_GATEWAY_PORT:-19444}
admin_password='Acceptance-Private-Password-2026!'

compose=(docker compose -p "$project" --env-file "$delivery/.env" -f "$delivery/docker-compose.yml")

cleanup() {
  if [[ -f $delivery/docker-compose.yml && -f $delivery/.env ]]; then
    "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT

for component in system core builder builder-worker api admin-ui agent-ui etcd-init migrate; do
  docker image inspect "$registry/$component:$version" >/dev/null || {
    echo "缺少Private验收镜像: $registry/$component:$version" >&2
    exit 1
  }
done

mkdir -p "$temporary/deploy"
cp -R "$repo_root/deploy/production" "$delivery"
cp -R "$repo_root/deploy/etcd" "$temporary/deploy/etcd"
mkdir -p "$delivery/secrets"

openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj '/CN=localhost' \
  -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' \
  -keyout "$delivery/secrets/tls.key" -out "$delivery/secrets/tls.crt" >/dev/null 2>&1
openssl ecparam -name prime256v1 -genkey -noout -out "$delivery/secrets/agent-ca.key"
openssl req -x509 -new -key "$delivery/secrets/agent-ca.key" -days 1 \
  -subj '/CN=AppForge Local Agent Acceptance CA' -addext 'basicConstraints=critical,CA:TRUE' \
  -addext 'keyUsage=critical,keyCertSign,cRLSign' -out "$delivery/secrets/agent-ca.crt" >/dev/null 2>&1
cp "$delivery/secrets/tls.crt" "$delivery/secrets/siem-ca.crt"
printf '%s\n' 'private-acceptance-siem-token' >"$delivery/secrets/siem-token"

(
  cd "$repo_root/appforge-api"
  GOCACHE=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache} \
    go run ./cmd/licensectl generate-key \
      --private "$temporary/vendor-private.pem" --public "$delivery/secrets/appforge-license-public.pem" >/dev/null
  GOCACHE=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache} \
    go run ./cmd/licensectl issue --private "$temporary/vendor-private.pem" \
      --output "$delivery/secrets/appforge-license.json" --license-id "$deployment_id" \
      --customer acceptance --deployment-id "$deployment_id" --modes "$deployment_mode" --valid-for 24h >/dev/null
)

bcrypt_base64=$(htpasswd -bnBC 10 '' "$admin_password" | tr -d ':\n' | base64 | tr -d '\n')
master_key=$(openssl rand -base64 32 | tr -d '\n')
cat >"$delivery/.env" <<EOF
APPFORGE_DEPLOYMENT_MODE=$deployment_mode
APPFORGE_VERSION=$version
APPFORGE_IMAGE_REGISTRY=$registry
APPFORGE_PUBLIC_ORIGIN=https://localhost:$agent_port
APPFORGE_ADMIN_HTTPS_PORT=$admin_port
APPFORGE_AGENT_HTTPS_PORT=$agent_port
APPFORGE_LOCAL_AGENT_GATEWAY_PORT=$gateway_port
APPFORGE_BUILDER_CONCURRENCY=1
APPFORGE_MYSQL_DATABASE=appforge
APPFORGE_MYSQL_USER=appforge
APPFORGE_MYSQL_PASSWORD=private_acceptance_mysql_password
APPFORGE_MYSQL_ROOT_PASSWORD=private_acceptance_mysql_root_password
APPFORGE_REDIS_PASSWORD=private_acceptance_redis_password
APPFORGE_MINIO_ACCESS_KEY=private_acceptance_minio
APPFORGE_MINIO_SECRET_KEY=private_acceptance_minio_secret
APPFORGE_INTERNAL_RPC_TOKEN=private_acceptance_internal_rpc_token
APPFORGE_JWT_ACCESS_SECRET=private_acceptance_jwt_secret_32_bytes
APPFORGE_SECRET_MASTER_KEY_BASE64=$master_key
APPFORGE_BOOTSTRAP_ADMIN_USERNAME=owner
APPFORGE_BOOTSTRAP_PASSWORD_BCRYPT_BASE64=$bcrypt_base64
APPFORGE_STRIPE_SECRET_KEY=
APPFORGE_STRIPE_WEBHOOK_SECRET=
APPFORGE_TLS_CERT_FILE=./secrets/tls.crt
APPFORGE_TLS_KEY_FILE=./secrets/tls.key
APPFORGE_LOCAL_AGENT_CA_CERT_FILE=./secrets/agent-ca.crt
APPFORGE_LOCAL_AGENT_CA_KEY_FILE=./secrets/agent-ca.key
APPFORGE_LICENSE_FILE=./secrets/appforge-license.json
APPFORGE_LICENSE_PUBLIC_KEY_FILE=./secrets/appforge-license-public.pem
APPFORGE_DEPLOYMENT_ID=$deployment_id
APPFORGE_SIEM_ENDPOINT=https://127.0.0.1:9/audit
APPFORGE_SIEM_TOKEN_FILE=./secrets/siem-token
APPFORGE_SIEM_CA_FILE=./secrets/siem-ca.crt
APPFORGE_LOCAL_SECRET_ROOT=
APPFORGE_KUBERNETES_SECRET_ROOT=
APPFORGE_VAULT_ADDRESS=
APPFORGE_VAULT_TOKEN_FILE=
APPFORGE_VAULT_NAMESPACE=
APPFORGE_AWS_REGION=
APPFORGE_AWS_SECRETS_MANAGER_ENDPOINT=
APPFORGE_BACKUP_DIR=./backups
APPFORGE_RPO_MINUTES=15
APPFORGE_RTO_MINUTES=120
EOF
chmod 0600 "$delivery/.env" "$delivery/secrets/tls.key" "$delivery/secrets/agent-ca.key" "$delivery/secrets/siem-token"
chmod 0644 "$delivery/secrets/"*.crt "$delivery/secrets/"*.pem "$delivery/secrets/appforge-license.json"

"$delivery/preflight.sh" "$delivery/.env" >/dev/null
if ! "${compose[@]}" up -d >"$temporary/up.log" 2>&1; then
  cat "$temporary/up.log" >&2
  "${compose[@]}" ps >&2 || true
  for service in mysql redis etcd etcd-init minio migrate; do
    echo "--- $service ---" >&2
    "${compose[@]}" logs --tail 80 "$service" >&2 || true
  done
  exit 1
fi

login_response=''
for _ in $(seq 1 90); do
  if curl -ksSf "https://127.0.0.1:$admin_port/" >/dev/null 2>&1 &&
    curl -ksSf "https://127.0.0.1:$agent_port/" >/dev/null 2>&1; then
    login_response=$(curl -ksSf -H 'Content-Type: application/json' \
      -d "{\"username\":\"owner\",\"password\":\"$admin_password\"}" \
      "https://127.0.0.1:$admin_port/admin/system/auth/login" 2>/dev/null || true)
    if [[ -n $login_response ]]; then
      break
    fi
  fi
  sleep 2
done

if [[ -z $login_response ]]; then
  "${compose[@]}" ps >&2 || true
  for service in migrate etcd-init system-rpc core-rpc builder-rpc api admin-web agent-web; do
    echo "--- $service ---" >&2
    "${compose[@]}" logs --tail 40 "$service" >&2 || true
  done
  echo "全新Private环境未能在时限内完成登录" >&2
  exit 1
fi

LOGIN_RESPONSE="$login_response" python3 -c '
import json, os
payload = json.loads(os.environ["LOGIN_RESPONSE"])
if payload.get("code") not in (0, 200) or not payload.get("data", {}).get("token"):
    raise SystemExit("启动管理员登录失败")
'

schema_present=$("${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -N -e "SELECT COUNT(*) FROM sys_schema_migration WHERE version=\"20260814_110_v4_builder_node_recovery\""')
[[ $schema_present == 1 ]] || {
  echo "Private安装缺少目标Schema版本" >&2
  exit 1
}

network="${project}_appforge-backend"
docker run --rm --network "$network" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c '
  mc alias set acceptance http://minio:9000 private_acceptance_minio private_acceptance_minio_secret >/dev/null
  mc stat acceptance/appforge >/dev/null
' || { echo "Private安装未初始化私有对象存储Bucket" >&2; exit 1; }

echo "通过: 全新${deployment_mode}环境迁移、非root服务、TLS双前端、许可证、启动管理员登录和对象存储初始化"
