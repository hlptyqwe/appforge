#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT

mkdir -p "$temporary/deploy"
cp -R "$repo_root/deploy/production" "$temporary/deploy/production"
cp -R "$repo_root/deploy/etcd" "$temporary/deploy/etcd"
production="$temporary/deploy/production"
mkdir -p "$production/secrets"

openssl req -x509 -newkey rsa:2048 -nodes -subj '/CN=appforge-preflight' -days 1 \
  -keyout "$production/secrets/tls.key" -out "$production/secrets/tls.crt" >/dev/null 2>&1
openssl ecparam -name prime256v1 -genkey -noout -out "$production/secrets/agent-ca.key"
openssl req -x509 -new -key "$production/secrets/agent-ca.key" -subj '/CN=appforge-agent-ca' -days 1 \
  -addext 'basicConstraints=critical,CA:TRUE' -addext 'keyUsage=critical,keyCertSign,cRLSign' \
  -out "$production/secrets/agent-ca.crt" >/dev/null 2>&1
cp "$production/secrets/tls.crt" "$production/secrets/appforge-license-public.pem"
cp "$production/secrets/tls.crt" "$production/secrets/siem-ca.crt"
printf '%s\n' '{"acceptance":"preflight-only"}' >"$production/secrets/appforge-license.json"
printf '%s\n' 'acceptance-siem-token' >"$production/secrets/siem-token"

printf '%s\n' \
  'APPFORGE_DEPLOYMENT_MODE=private' \
  'APPFORGE_VERSION=1.1.0' \
  'APPFORGE_IMAGE_REGISTRY=registry.example.com/appforge' \
  'APPFORGE_PUBLIC_ORIGIN=https://appforge.example.com' \
  'APPFORGE_BUILDER_CONCURRENCY=2' \
  'APPFORGE_MYSQL_DATABASE=appforge' \
  'APPFORGE_MYSQL_USER=appforge' \
  'APPFORGE_MYSQL_PASSWORD=mysql_password_1234567890' \
  'APPFORGE_MYSQL_ROOT_PASSWORD=mysql_root_password_123456' \
  'APPFORGE_REDIS_PASSWORD=redis_password_1234567890' \
  'APPFORGE_MINIO_ACCESS_KEY=minio_access_1234567890' \
  'APPFORGE_MINIO_SECRET_KEY=minio_secret_12345678901' \
  'APPFORGE_INTERNAL_RPC_TOKEN=internal_rpc_12345678901' \
  'APPFORGE_JWT_ACCESS_SECRET=jwt_access_secret_1234567890' \
  'APPFORGE_SECRET_MASTER_KEY_BASE64=YWNjZXB0YW5jZV9tYXN0ZXJfa2V5XzMyX2J5dGVz' \
  'APPFORGE_BOOTSTRAP_ADMIN_USERNAME=owner' \
  'APPFORGE_BOOTSTRAP_PASSWORD_BCRYPT_BASE64=YWNjZXB0YW5jZS1iY3J5cHQ=' \
  'APPFORGE_TLS_CERT_FILE=./secrets/tls.crt' \
  'APPFORGE_TLS_KEY_FILE=./secrets/tls.key' \
  'APPFORGE_LOCAL_AGENT_CA_CERT_FILE=./secrets/agent-ca.crt' \
  'APPFORGE_LOCAL_AGENT_CA_KEY_FILE=./secrets/agent-ca.key' \
  'APPFORGE_LICENSE_FILE=./secrets/appforge-license.json' \
  'APPFORGE_LICENSE_PUBLIC_KEY_FILE=./secrets/appforge-license-public.pem' \
  'APPFORGE_DEPLOYMENT_ID=acceptance-private' \
  'APPFORGE_SIEM_ENDPOINT=https://siem.example.com/appforge/audit' \
  'APPFORGE_SIEM_TOKEN_FILE=./secrets/siem-token' \
  'APPFORGE_SIEM_CA_FILE=./secrets/siem-ca.crt' \
  >"$production/.env"
chmod 0600 "$production/.env" "$production/secrets/tls.key" "$production/secrets/agent-ca.key" "$production/secrets/siem-token"
chmod 0644 "$production/secrets/"*.crt "$production/secrets/"*.pem "$production/secrets/appforge-license.json"

"$production/preflight.sh" "$production/.env" >/dev/null

docker compose --env-file "$production/.env" -f "$production/docker-compose.yml" config --format json |
  python3 -c '
import json, sys
config = json.load(sys.stdin)
services = config["services"]
init = services["minio-init"]
if init.get("user") != "65532:65532" or not init.get("read_only"):
    raise SystemExit("minio-init必须以固定非root用户和只读根文件系统运行")
if "ALL" not in init.get("cap_drop", []):
    raise SystemExit("minio-init必须drop全部capabilities")
for name in ("api", "builder-worker"):
    condition = services[name].get("depends_on", {}).get("minio-init", {}).get("condition")
    if condition != "service_completed_successfully":
        raise SystemExit(name + "未等待Bucket初始化成功")
'

cp "$production/secrets/agent-ca.key" "$temporary/agent-ca.key"
cp "$production/secrets/agent-ca.crt" "$temporary/agent-ca.crt"
cp "$production/secrets/tls.key" "$production/secrets/agent-ca.key"
cp "$production/secrets/tls.crt" "$production/secrets/agent-ca.crt"
chmod 0600 "$production/secrets/agent-ca.key"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了RSA Local Agent CA" >&2
  exit 1
fi
cp "$temporary/agent-ca.key" "$production/secrets/agent-ca.key"
cp "$temporary/agent-ca.crt" "$production/secrets/agent-ca.crt"
chmod 0600 "$production/secrets/agent-ca.key"

cp "$production/secrets/tls.key" "$temporary/tls.key"
cp "$production/secrets/agent-ca.key" "$production/secrets/tls.key"
chmod 0600 "$production/secrets/tls.key"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了不匹配的TLS证书与私钥" >&2
  exit 1
fi
cp "$temporary/tls.key" "$production/secrets/tls.key"
chmod 0600 "$production/secrets/tls.key"

chmod 0644 "$production/.env"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了0644的.env" >&2
  exit 1
fi
chmod 0600 "$production/.env"
chmod 0644 "$production/secrets/tls.key"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了0644的TLS私钥" >&2
  exit 1
fi

echo "通过: 生产配置渲染、Compose解析、证书/ECDSA CA及.env/私钥权限门禁"
