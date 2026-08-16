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
  'APPFORGE_VERSION=1.2.0' \
  'APPFORGE_SCHEMA_VERSION=20260815_113_v7_air_gapped' \
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

cp "$production/.env" "$temporary/https-siem.env"
sed -i.bak 's#APPFORGE_SIEM_ENDPOINT=https://siem.example.com/appforge/audit#APPFORGE_SIEM_ENDPOINT=syslog+tls://siem.customer.test:6514#' "$production/.env"
sed -i.bak '/^APPFORGE_SIEM_TOKEN_FILE=/d' "$production/.env"
printf '%s\n' 'APPFORGE_SIEM_SYSLOG_HOSTNAME=api-acceptance' 'APPFORGE_SIEM_SYSLOG_APP_NAME=appforge-admin' >>"$production/.env"
"$production/preflight.sh" "$production/.env" >/dev/null
docker compose --env-file "$production/.env" -f "$production/docker-compose.yml" config --format json |
  python3 -c '
import json, sys
environment = json.load(sys.stdin)["services"]["api"]["environment"]
if environment.get("APPFORGE_SIEM_ENDPOINT") != "syslog+tls://siem.customer.test:6514":
    raise SystemExit("API未接入RFC5424/TLS SIEM Endpoint")
if environment.get("APPFORGE_SIEM_SYSLOG_HOSTNAME") != "api-acceptance":
    raise SystemExit("API未接入Syslog hostname")
if environment.get("APPFORGE_SIEM_SYSLOG_APP_NAME") != "appforge-admin":
    raise SystemExit("API未接入Syslog app-name")
if environment.get("APPFORGE_SIEM_TOKEN_FILE") != "/etc/appforge/siem/token":
    raise SystemExit("Compose未提供安全的无Token占位挂载路径")
'
cp "$production/.env" "$temporary/syslog-valid.env"
sed -i.bak 's#syslog+tls://siem.customer.test:6514#syslog+tls://siem.customer.test:6514/events#' "$production/.env"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了带路径的Syslog TLS Endpoint" >&2
  exit 1
fi
cp "$temporary/syslog-valid.env" "$production/.env"
sed -i.bak 's#syslog+tls://siem.customer.test:6514#syslog://siem.customer.test:514#' "$production/.env"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了明文Syslog Endpoint" >&2
  exit 1
fi
cp "$temporary/https-siem.env" "$production/.env"
chmod 0600 "$production/.env"

cp "$production/.env" "$temporary/schema-valid.env"
sed -i.bak 's/APPFORGE_SCHEMA_VERSION=20260815_113_v7_air_gapped/APPFORGE_SCHEMA_VERSION=20260815_112_v7_customer_storage/' "$production/.env"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了1.2.x与Schema 112的不兼容组合" >&2
  exit 1
fi
cp "$temporary/schema-valid.env" "$production/.env"
chmod 0600 "$production/.env"

cp "$production/.env" "$temporary/base.env"
printf '%s\n' 'siem.customer.test:443' 'collector.customer.test:4318' >"$production/secrets/egress-allowlist.txt"
chmod 0444 "$production/secrets/egress-allowlist.txt"
printf '%s\n' \
  'APPFORGE_EGRESS_PROXY_ENABLED=true' \
  'APPFORGE_EGRESS_PROXY_URL=http://egress-proxy:3128' \
  'APPFORGE_EGRESS_PROXY_ALLOWLIST_FILE=./secrets/egress-allowlist.txt' \
  'APPFORGE_EGRESS_PROXY_MAX_CONNECTIONS=128' \
  'APPFORGE_EGRESS_NO_PROXY=localhost,127.0.0.1,mysql,redis,etcd,minio,system-rpc,core-rpc,builder-rpc' >>"$production/.env"
"$production/preflight.sh" "$production/.env" >/dev/null
docker compose --env-file "$production/.env" -f "$production/docker-compose.yml" --profile egress config --format json |
  python3 -c '
import json, sys
services = json.load(sys.stdin)["services"]
proxy = services["egress-proxy"]
if set(proxy.get("networks", {})) != {"appforge-backend", "appforge-egress"}:
    raise SystemExit("出口代理必须同时连接内部与出口网络")
for name in ("api", "builder-worker", "core-rpc", "source-trigger-worker"):
    environment = services[name].get("environment", {})
    if environment.get("HTTPS_PROXY") != "http://egress-proxy:3128":
        raise SystemExit(name + " 未接入受限出口代理")
'
chmod 0644 "$production/secrets/egress-allowlist.txt"
printf '%s\n' 'siem.example.com:443' >"$production/secrets/egress-allowlist.txt"
chmod 0444 "$production/secrets/egress-allowlist.txt"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了出口Allowlist占位域名" >&2
  exit 1
fi
cp "$temporary/base.env" "$production/.env"
chmod 0600 "$production/.env"

printf '%s\n' \
  'APPFORGE_REPLICA_ENDPOINT=https://replica.example.com' \
  'APPFORGE_REPLICA_ACCESS_KEY=replica_access_key' \
  'APPFORGE_REPLICA_SECRET_KEY=replica_secret_password_12345' \
  'APPFORGE_REPLICA_BUCKET=appforge' \
  'APPFORGE_REPLICA_RULE_ID=appforge-dr' \
  'APPFORGE_REPLICA_SYNC=false' >>"$production/.env"
"$production/preflight.sh" "$production/.env" >/dev/null
cp "$temporary/base.env" "$production/.env"
printf '%s\n' \
  'APPFORGE_REPLICA_ENDPOINT=http://replica.example.com' \
  'APPFORGE_REPLICA_ACCESS_KEY=replica_access_key' \
  'APPFORGE_REPLICA_SECRET_KEY=replica_secret_password_12345' >>"$production/.env"
if "$production/preflight.sh" "$production/.env" >/dev/null 2>&1; then
  echo "验收失败: preflight接受了HTTP对象副本Endpoint" >&2
  exit 1
fi
cp "$temporary/base.env" "$production/.env"
chmod 0600 "$production/.env"

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
for name in ("api", "system-rpc", "core-rpc", "builder-rpc", "builder-worker", "webhook-worker", "billing-worker", "enterprise-worker", "source-trigger-worker"):
    environment = services[name].get("environment", {})
    if environment.get("APPFORGE_PROMETHEUS_ENABLED") != "true":
        raise SystemExit(name + "未启用Prometheus")
    if str(environment.get("APPFORGE_PROMETHEUS_PORT")) != "9101" or environment.get("APPFORGE_PROMETHEUS_PATH") != "/metrics":
        raise SystemExit(name + "的Prometheus端口或路径错误")
    if environment.get("APPFORGE_OTLP_SAMPLER") not in ("0.1", 0.1):
        raise SystemExit(name + "的OTLP采样率错误")
'
grep -Fq 'Encoding: json' "$production/runtime/common.yaml" || {
  echo "验收失败: 生产运行配置未启用JSON结构化日志" >&2
  exit 1
}

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
