#!/usr/bin/env bash

set -euo pipefail

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
env_file=${1:-$delivery_dir/.env}
repo_deploy_dir=$(cd "$delivery_dir/.." && pwd)
admin_api_template="$delivery_dir/admin-api.yaml.template"
if [[ ! -f $admin_api_template ]]; then
  admin_api_template="$repo_deploy_dir/etcd/admin-api.yaml"
fi

[[ -f "$env_file" ]] || { echo "缺少生产配置: $env_file" >&2; exit 1; }
set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

umask 077
runtime_dir="$delivery_dir/runtime"
mkdir -p "$runtime_dir"

cat > "$runtime_dir/common.yaml" <<EOF
Log:
  ServiceName: appforge
  Mode: console
  Encoding: json
  Level: info
InternalRpc:
  Token: ${APPFORGE_INTERNAL_RPC_TOKEN}
EOF

cat > "$runtime_dir/system.yaml" <<EOF
Name: system.rpc
ListenOn: 0.0.0.0:8080
Mode: pro
Etcd:
  Hosts: [etcd:2379]
  Key: system.rpc
Mysql:
  DataSource: ${APPFORGE_MYSQL_USER}:${APPFORGE_MYSQL_PASSWORD}@tcp(mysql:3306)/${APPFORGE_MYSQL_DATABASE}?charset=utf8mb4&parseTime=true&loc=Asia%2FHong_Kong
CacheRedis:
  - Host: redis:6379
    Type: node
    Pass: ${APPFORGE_REDIS_PASSWORD}
Jwt:
  AccessSecret: ${APPFORGE_JWT_ACCESS_SECRET}
  AccessExpire: 86400
EOF

cat > "$runtime_dir/core.yaml" <<EOF
Name: core.rpc
ListenOn: 0.0.0.0:8081
Mode: pro
Etcd:
  Hosts: [etcd:2379]
  Key: core.rpc
Mysql:
  DataSource: ${APPFORGE_MYSQL_USER}:${APPFORGE_MYSQL_PASSWORD}@tcp(mysql:3306)/${APPFORGE_MYSQL_DATABASE}?charset=utf8mb4&parseTime=true&loc=Asia%2FHong_Kong
CacheRedis:
  - Host: redis:6379
    Type: node
    Pass: ${APPFORGE_REDIS_PASSWORD}
SigningSecrets:
  MasterKeyBase64: ${APPFORGE_SECRET_MASTER_KEY_BASE64}
WebhookWorker:
  Enabled: true
  PollInterval: 1s
  HttpTimeout: 10s
  BatchSize: 20
BillingWorker:
  Enabled: true
  PollInterval: 30s
EnterpriseWorker:
  Enabled: true
  PollInterval: 15s
  OfflineAfter: 90s
AgentPKI:
  CACertificateFile: /etc/appforge/agent-ca.crt
  CAPrivateKeyFile: /etc/appforge/agent-ca.key
  CertificateTTL: 24h
EOF

cat > "$runtime_dir/builder.yaml" <<EOF
Name: builder.rpc
ListenOn: 0.0.0.0:8082
Mode: pro
Middlewares:
  Stat: false
Etcd:
  Hosts: [etcd:2379]
  Key: builder.rpc
CoreRpc:
  Target: core-rpc:8081
BuilderRpc:
  Target: builder-rpc:8082
Builder:
  Id: builder-production
  PoolCode: default
  Endpoint: docker
  MaxConcurrency: ${APPFORGE_BUILDER_CONCURRENCY:-2}
  LeaseSeconds: 120
  PollInterval: 2s
  NodeHeartbeat: 15s
  TempDir: /tmp/appforge-builder
  ToolchainVersion: android-enterprise-v1
  BuildProtocolVersion: 1
  CapabilityJson: '{"apk":true,"branding":true,"whiteLabel":true,"cache":true}'
  CacheTtl: 168h
ObjectCleanup:
  Interval: 10s
  StaleAfter: 1h
  BatchSize: 100
ObjectStorage:
  OssType: 3
  Minio:
    Endpoint: http://minio:9000
    BucketUrl: ${APPFORGE_PUBLIC_ORIGIN}/appforge
    AccessKeyId: ${APPFORGE_MINIO_ACCESS_KEY}
    AccessKeySecret: ${APPFORGE_MINIO_SECRET_KEY}
    BucketName: appforge
SigningSecrets:
  MasterKeyBase64: ${APPFORGE_SECRET_MASTER_KEY_BASE64}
SecretProviders:
  MaxSecretBytes: 65536
  LocalRoot: ${APPFORGE_LOCAL_SECRET_ROOT:-}
  KubernetesRoot: ${APPFORGE_KUBERNETES_SECRET_ROOT:-}
  Vault:
    Address: ${APPFORGE_VAULT_ADDRESS:-}
    TokenFile: ${APPFORGE_VAULT_TOKEN_FILE:-}
    Namespace: ${APPFORGE_VAULT_NAMESPACE:-}
    AllowHTTP: false
  AWS:
    Region: ${APPFORGE_AWS_REGION:-}
    Endpoint: ${APPFORGE_AWS_SECRETS_MANAGER_ENDPOINT:-}
EOF

[[ -f $admin_api_template && ! -L $admin_api_template ]] || {
  echo "缺少 Admin API 配置模板: $admin_api_template" >&2
  exit 1
}
cp "$admin_api_template" "$runtime_dir/admin-api.yaml"
public_origin=${APPFORGE_PUBLIC_ORIGIN//\//\\/}
jwt_secret=${APPFORGE_JWT_ACCESS_SECRET//\//\\/}
master_key=${APPFORGE_SECRET_MASTER_KEY_BASE64//\//\\/}
stripe_key=${APPFORGE_STRIPE_SECRET_KEY:-}
stripe_webhook=${APPFORGE_STRIPE_WEBHOOK_SECRET:-}
sed -i.bak \
  -e 's/^Mode: dev$/Mode: pro/' \
  -e "s/^  AccessSecret: .*/  AccessSecret: ${jwt_secret}/" \
  -e "s/^  MasterKeyBase64: .*/  MasterKeyBase64: ${master_key}/" \
  -e "s#http:\/\/localhost:5174#${public_origin}#g" \
  -e "s#http:\/\/localhost:8888#${public_origin}#g" \
  -e "s/^  StripeSecretKey: .*/  StripeSecretKey: ${stripe_key}/" \
  -e "s/^  StripeWebhookSecret: .*/  StripeWebhookSecret: ${stripe_webhook}/" \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s/^  Enabled: false/  Enabled: true/' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ServerCertificate:.*#  ServerCertificate: /etc/appforge/tls/tls.crt#' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ServerPrivateKey:.*#  ServerPrivateKey: /etc/appforge/tls/tls.key#' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ClientCACertificate:.*#  ClientCACertificate: /etc/appforge/agent-ca.crt#' \
  -e '/^OfflineLicense:/,/^Audit:/ s/^  Enabled: false/  Enabled: true/' \
  -e '/^OfflineLicense:/,/^Audit:/ s#^  LicenseFile:.*#  LicenseFile: /etc/appforge/license/license.json#' \
  -e '/^OfflineLicense:/,/^Audit:/ s#^  PublicKeyFile:.*#  PublicKeyFile: /etc/appforge/license/vendor-public.pem#' \
  -e '/^OfflineLicense:/,/^Audit:/ s#^  StateFile:.*#  StateFile: /var/lib/appforge-license/state.json#' \
  -e "/^OfflineLicense:/,/^Audit:/ s#^  DeploymentId:.*#  DeploymentId: ${APPFORGE_DEPLOYMENT_ID}#" \
  -e "/^OfflineLicense:/,/^Audit:/ s#^  DeploymentMode:.*#  DeploymentMode: ${APPFORGE_DEPLOYMENT_MODE}#" \
  "$runtime_dir/admin-api.yaml"
rm -f "$runtime_dir/admin-api.yaml.bak"
chmod 600 "$runtime_dir"/*.yaml

echo "生产运行配置已生成到 ${runtime_dir}（内容包含 Secret，请勿归档或提交）"
