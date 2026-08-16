#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
registry=${APPFORGE_PRIVATE_ACCEPTANCE_REGISTRY:-appforge-private-acceptance}
version=${APPFORGE_PRIVATE_ACCEPTANCE_VERSION:-1.2.0}
deployment_mode=${APPFORGE_PRIVATE_ACCEPTANCE_MODE:-private}
case "$deployment_mode" in private|dedicated) ;; *) echo "安装验收模式只能是private或dedicated" >&2; exit 2;; esac
deployment_id="${deployment_mode}-install-acceptance"
project="appforge-v7-${deployment_mode}-${RANDOM}-$$"
temporary=$(mktemp -d)
delivery="$temporary/deploy/production"
admin_port=${APPFORGE_PRIVATE_ACCEPTANCE_ADMIN_PORT:-18443}
agent_port=${APPFORGE_PRIVATE_ACCEPTANCE_AGENT_PORT:-19443}
gateway_port=${APPFORGE_PRIVATE_ACCEPTANCE_GATEWAY_PORT:-19444}
offline_mode=${APPFORGE_PRIVATE_ACCEPTANCE_OFFLINE_MODE:-0}
formal_release_media=${APPFORGE_PRIVATE_ACCEPTANCE_FORMAL_RELEASE_MEDIA:-0}
formal_release_platform=${APPFORGE_PRIVATE_ACCEPTANCE_FORMAL_RELEASE_PLATFORM:-linux/amd64}
report_file=${APPFORGE_PRIVATE_ACCEPTANCE_REPORT_FILE:-}
migrate_image=${APPFORGE_PRIVATE_ACCEPTANCE_MIGRATE_IMAGE:-$registry/migrate:$version}
migrate_image_id=$(docker image inspect -f '{{.Id}}' "$migrate_image" 2>/dev/null || true)
upgrade_mode=${APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_MODE:-0}
upgrade_version=${APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_VERSION:-1.2.1}
upgrade_max_consecutive_failures=${APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_MAX_CONSECUTIVE_FAILURES:-10}
admin_password='Acceptance-Private-Password-2026!'
upgrade_probe_container=''
upgrade_tagged_components=()
mc_image=${APPFORGE_PRIVATE_ACCEPTANCE_MC_IMAGE:-minio/mc:RELEASE.2025-04-16T18-13-26Z}

case "$offline_mode" in 0|1) ;; *) echo "APPFORGE_PRIVATE_ACCEPTANCE_OFFLINE_MODE 只能是0或1" >&2; exit 2;; esac
case "$formal_release_media" in 0|1) ;; *) echo "APPFORGE_PRIVATE_ACCEPTANCE_FORMAL_RELEASE_MEDIA 只能是0或1" >&2; exit 2;; esac
case "$upgrade_mode" in 0|1) ;; *) echo "APPFORGE_PRIVATE_ACCEPTANCE_UPGRADE_MODE 只能是0或1" >&2; exit 2;; esac
case "$formal_release_platform" in linux/amd64|linux/arm64) ;; *) echo "正式介质平台不合法" >&2; exit 2;; esac
[[ $migrate_image =~ ^[A-Za-z0-9._:/-]+$ ]] || { echo "迁移镜像引用不安全" >&2; exit 2; }
[[ $upgrade_version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "升级版本必须是语义化版本" >&2; exit 2; }
[[ $upgrade_max_consecutive_failures =~ ^[0-9]+$ ]] || { echo "升级连续失败门禁必须是非负整数" >&2; exit 2; }
if [[ $upgrade_mode == 1 && $offline_mode != 1 ]]; then
  echo "离线升级验收必须启用断网模式" >&2
  exit 2
fi
if [[ $formal_release_media == 1 && $offline_mode != 1 ]]; then
  echo "正式发布介质验收必须启用断网模式" >&2
  exit 2
fi
if [[ $formal_release_media == 1 ]]; then
  mc_image="$registry/minio-mc:$version"
  export DOCKER_DEFAULT_PLATFORM="$formal_release_platform"
fi

compose=(docker compose -p "$project" --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" -f "$delivery/acceptance.override.yml")
if [[ $offline_mode == 1 ]]; then
  compose+=(-f "$delivery/offline-network.override.yml")
fi

cleanup() {
  if [[ -n $upgrade_probe_container ]]; then
    docker rm -f "$upgrade_probe_container" >/dev/null 2>&1 || true
  fi
  if [[ -f $delivery/docker-compose.yml && -f $delivery/.env ]]; then
    "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  for component in "${upgrade_tagged_components[@]}"; do
    docker image rm "$registry/$component:$upgrade_version" >/dev/null 2>&1 || true
  done
  rm -rf "$temporary"
}
trap cleanup EXIT

internal_login() {
  local attempts=${1:-90}
  local frontend_network="${project}_appforge-frontend"
  local login_payload="{\"username\":\"owner\",\"password\":\"$admin_password\"}"
  docker run --rm --pull never --network "$frontend_network" \
    --entrypoint /bin/sh -e LOGIN_PAYLOAD="$login_payload" -e LOGIN_ATTEMPTS="$attempts" alpine:3.22 -ec '
      for _ in $(seq 1 "$LOGIN_ATTEMPTS"); do
        if wget --no-check-certificate -qO- -T 3 https://admin-web:8443/ >/dev/null 2>&1 &&
          wget --no-check-certificate -qO- -T 3 https://agent-web:8443/ >/dev/null 2>&1; then
          response=$(wget --no-check-certificate -qO- -T 3 \
            --header="Content-Type: application/json" --post-data="$LOGIN_PAYLOAD" \
            https://admin-web:8443/admin/system/auth/login 2>/dev/null || true)
          if [ -n "$response" ]; then
            printf "%s" "$response"
            exit 0
          fi
        fi
        sleep 2
      done
      exit 1
    '
}

validate_login_response() {
  LOGIN_RESPONSE="$1" python3 -c '
import json, os
payload = json.loads(os.environ["LOGIN_RESPONSE"])
if payload.get("code") not in (0, 200) or not payload.get("data", {}).get("token"):
    raise SystemExit("启动管理员登录失败")
'
}

assert_service_image() {
  local service=$1 expected_image=$2 container_id actual_image
  container_id=$("${compose[@]}" ps -q "$service")
  [[ -n $container_id ]] || { echo "验收失败: 服务 $service 没有运行容器" >&2; return 1; }
  actual_image=$(docker inspect -f '{{.Config.Image}}' "$container_id")
  [[ $actual_image == "$expected_image" ]] || {
    echo "验收失败: 服务 $service 镜像=$actual_image，期望=$expected_image" >&2
    return 1
  }
}

for component in system core builder builder-worker api admin-ui agent-ui etcd-init; do
  docker image inspect "$registry/$component:$version" >/dev/null || {
    echo "缺少Private验收镜像: $registry/$component:$version" >&2
    exit 1
  }
done
docker image inspect "$migrate_image" >/dev/null || {
  echo "缺少Private验收迁移镜像: $migrate_image" >&2
  exit 1
}
if [[ $offline_mode == 1 ]]; then
  offline_images=(redis:7.4-alpine alpine:3.22)
  if [[ $formal_release_media == 1 ]]; then
    for component in system core builder builder-worker api admin-ui agent-ui local-agent egress-proxy mysql etcd minio minio-mc etcd-init migrate mysql-binlog-tools; do
      offline_images+=("$registry/$component:$version")
    done
  else
    offline_images+=(mysql:8.4 quay.io/coreos/etcd:v3.6.12 \
      minio/minio:RELEASE.2025-04-22T22-12-26Z minio/mc:RELEASE.2025-04-16T18-13-26Z)
  fi
  for image in "${offline_images[@]}"; do
    docker image inspect "$image" >/dev/null || {
      echo "缺少离线验收预载镜像: $image" >&2
      exit 1
    }
    if [[ $formal_release_media == 1 ]]; then
      actual_platform=$(docker image inspect -f '{{.Os}}/{{.Architecture}}' "$image")
      [[ $actual_platform == "$formal_release_platform" ]] || {
        echo "正式介质镜像平台错误: $image=$actual_platform，期望=$formal_release_platform" >&2
        exit 1
      }
    fi
  done
fi

mkdir -p "$temporary/deploy"
cp -R "$repo_root/deploy/production" "$delivery"
cp -R "$repo_root/deploy/etcd" "$temporary/deploy/etcd"
mkdir -p "$delivery/secrets"
cat >"$delivery/acceptance.override.yml" <<YAML
services:
  migrate:
    image: $migrate_image
YAML
if [[ $offline_mode == 1 ]]; then
  cat >"$delivery/offline-network.override.yml" <<YAML
networks:
  appforge-frontend:
    internal: true
  appforge-backend:
    internal: true
YAML
fi

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
APPFORGE_SCHEMA_VERSION=20260815_113_v7_air_gapped
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
if ! "${compose[@]}" up -d --pull never >"$temporary/up.log" 2>&1; then
  cat "$temporary/up.log" >&2
  "${compose[@]}" ps >&2 || true
  for service in mysql redis etcd etcd-init minio migrate; do
    echo "--- $service ---" >&2
    "${compose[@]}" logs --tail 80 "$service" >&2 || true
  done
  exit 1
fi

login_response=''
if [[ $offline_mode == 1 ]]; then
  frontend_network="${project}_appforge-frontend"
  login_response=$(internal_login 90 || true)
else
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
fi

if [[ -z $login_response ]]; then
  "${compose[@]}" ps >&2 || true
  for service in migrate etcd-init system-rpc core-rpc builder-rpc api admin-web agent-web; do
    echo "--- $service ---" >&2
    "${compose[@]}" logs --tail 40 "$service" >&2 || true
  done
  echo "全新Private环境未能在时限内完成登录" >&2
  exit 1
fi

validate_login_response "$login_response"

schema_state=$("${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -N -e "SELECT CONCAT(SUM(version=\"20260815_112_v7_customer_storage\"),\":\",SUM(version=\"20260815_113_v7_air_gapped\"),\":\",(SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=\"t_air_gapped_package\")) FROM sys_schema_migration"')
[[ $schema_state == 1:1:1 ]] || {
  echo "Private安装 Schema 边界错误，期望112和113迁移及AIR_GAPPED表均存在，实际=$schema_state" >&2
  exit 1
}
schema_present=$("${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -N -e "SELECT COUNT(*) FROM sys_schema_migration WHERE version=\"20260814_110_v4_builder_node_recovery\""')
[[ $schema_present == 1 ]] || {
  echo "Private安装缺少目标Schema版本" >&2
  exit 1
}

network="${project}_appforge-backend"
docker run --rm --pull never --network "$network" \
  --tmpfs /tmp:size=16m,mode=0700,uid=65532,gid=65532 -e MC_CONFIG_DIR=/tmp/mc \
  --entrypoint /bin/sh "$mc_image" -c '
  mc alias set acceptance http://minio:9000 private_acceptance_minio private_acceptance_minio_secret >/dev/null
  mc stat acceptance/appforge >/dev/null
  version_status=$(mc version info acceptance/appforge)
  case "$version_status" in *enabled*) ;; *) exit 1 ;; esac
' || { echo "Private安装未初始化私有对象存储Bucket或版本化" >&2; exit 1; }

if [[ $offline_mode == 1 ]]; then
  [[ $(docker network inspect -f '{{.Internal}}' "$frontend_network") == true ]] || {
    echo "离线安装前端网络仍允许外部路由" >&2
    exit 1
  }
  [[ $(docker network inspect -f '{{.Internal}}' "$network") == true ]] || {
    echo "离线安装后端网络仍允许外部路由" >&2
    exit 1
  }
  docker run --rm --network "$frontend_network" --entrypoint /bin/sh alpine:3.22 -ec '
    wget -qO- -T 3 http://api:8888/healthz | grep -q '"'"'"status":"ok"'"'"'
    if wget -qO- -T 3 https://example.com >/dev/null 2>&1; then
      echo "离线网络仍可访问外部HTTPS" >&2
      exit 1
    fi
  '
fi

upgrade_completed=false
rollback_completed=false
upgrade_duration_seconds=0
rollback_duration_seconds=0
upgrade_probe_requests=0
upgrade_probe_failures=0
upgrade_probe_max_consecutive_failures=0
if [[ $upgrade_mode == 1 ]]; then
  upgrade_components=(system core builder builder-worker api admin-ui agent-ui etcd-init)
  for component in "${upgrade_components[@]}"; do
    if docker image inspect "$registry/$component:$upgrade_version" >/dev/null 2>&1; then
      echo "验收失败: 升级测试标签已存在，拒绝覆盖 $registry/$component:$upgrade_version" >&2
      exit 1
    fi
    docker image tag "$registry/$component:$version" "$registry/$component:$upgrade_version"
    upgrade_tagged_components+=("$component")
  done

  "${compose[@]}" exec -T mysql sh -c \
    'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -e "CREATE TABLE acceptance_offline_upgrade_marker(id BIGINT PRIMARY KEY,value VARCHAR(64) NOT NULL); INSERT INTO acceptance_offline_upgrade_marker(id,value) VALUES (1,\"database-preserved\");"'
  docker run --rm --pull never --network "$network" \
    --tmpfs /tmp:size=16m,mode=0700,uid=65532,gid=65532 -e MC_CONFIG_DIR=/tmp/mc --entrypoint /bin/sh \
    "$mc_image" -ec '
      mc alias set acceptance http://minio:9000 private_acceptance_minio private_acceptance_minio_secret >/dev/null
      printf "%s" object-preserved | mc pipe acceptance/appforge/acceptance/offline-upgrade-marker.txt >/dev/null
    '

  mkdir -p "$temporary/upgrade-probe"
  chmod 0777 "$temporary/upgrade-probe"
  upgrade_probe_container="${project}-upgrade-probe"
  docker run -d --pull never --name "$upgrade_probe_container" --network "$frontend_network" \
    -v "$temporary/upgrade-probe:/probe" --entrypoint /bin/sh alpine:3.22 -ec '
      while :; do
        if wget -qO- -T 2 http://api:8888/healthz >/dev/null 2>&1; then
          echo ok >>/probe/results
        else
          echo failed >>/probe/results
        fi
        sleep 0.2
      done
    ' >/dev/null
  sleep 2

  upgrade_started_ns=$(python3 -c 'import time; print(time.time_ns())')
  sed -i.bak "s/^APPFORGE_VERSION=.*/APPFORGE_VERSION=$upgrade_version/" "$delivery/.env"
  rm -f "$delivery/.env.bak"
  if ! "${compose[@]}" up -d --pull never >"$temporary/upgrade.log" 2>&1; then
    tail -160 "$temporary/upgrade.log" >&2
    exit 1
  fi
  upgraded_login=$(internal_login 90 || true)
  [[ -n $upgraded_login ]] || { echo "验收失败: 离线升级后管理员登录不可用" >&2; exit 1; }
  validate_login_response "$upgraded_login"
  for mapping in \
    system-rpc:system core-rpc:core builder-rpc:builder builder-worker:builder-worker \
    api:api webhook-worker:core billing-worker:core enterprise-worker:core \
    source-trigger-worker:api admin-web:admin-ui agent-web:agent-ui; do
    assert_service_image "${mapping%%:*}" "$registry/${mapping#*:}:$upgrade_version"
  done
  upgrade_completed_ns=$(python3 -c 'import time; print(time.time_ns())')
  upgrade_duration_seconds=$(python3 -c \
    'import sys; print(f"{(int(sys.argv[2])-int(sys.argv[1]))/1e9:.6f}")' \
    "$upgrade_started_ns" "$upgrade_completed_ns")

  database_marker=$("${compose[@]}" exec -T mysql sh -c \
    'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -N -e "SELECT value FROM acceptance_offline_upgrade_marker WHERE id=1"')
  object_marker=$(docker run --rm --pull never --network "$network" \
    --tmpfs /tmp:size=16m,mode=0700,uid=65532,gid=65532 -e MC_CONFIG_DIR=/tmp/mc --entrypoint /bin/sh \
    "$mc_image" -ec '
      mc alias set acceptance http://minio:9000 private_acceptance_minio private_acceptance_minio_secret >/dev/null
      mc cat acceptance/appforge/acceptance/offline-upgrade-marker.txt
    ')
  [[ $database_marker == database-preserved && $object_marker == object-preserved ]] || {
    echo "验收失败: 离线升级后持久数据不一致" >&2
    exit 1
  }
  upgrade_completed=true

  rollback_started_ns=$(python3 -c 'import time; print(time.time_ns())')
  sed -i.bak "s/^APPFORGE_VERSION=.*/APPFORGE_VERSION=$version/" "$delivery/.env"
  rm -f "$delivery/.env.bak"
  if ! "${compose[@]}" up -d --pull never >"$temporary/rollback.log" 2>&1; then
    tail -160 "$temporary/rollback.log" >&2
    exit 1
  fi
  rollback_login=$(internal_login 90 || true)
  [[ -n $rollback_login ]] || { echo "验收失败: 应用回滚后管理员登录不可用" >&2; exit 1; }
  validate_login_response "$rollback_login"
  for mapping in \
    system-rpc:system core-rpc:core builder-rpc:builder builder-worker:builder-worker \
    api:api webhook-worker:core billing-worker:core enterprise-worker:core \
    source-trigger-worker:api admin-web:admin-ui agent-web:agent-ui; do
    assert_service_image "${mapping%%:*}" "$registry/${mapping#*:}:$version"
  done
  rollback_completed_ns=$(python3 -c 'import time; print(time.time_ns())')
  rollback_duration_seconds=$(python3 -c \
    'import sys; print(f"{(int(sys.argv[2])-int(sys.argv[1]))/1e9:.6f}")' \
    "$rollback_started_ns" "$rollback_completed_ns")
  database_marker=$("${compose[@]}" exec -T mysql sh -c \
    'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -N -e "SELECT value FROM acceptance_offline_upgrade_marker WHERE id=1"')
  object_marker=$(docker run --rm --pull never --network "$network" \
    --tmpfs /tmp:size=16m,mode=0700,uid=65532,gid=65532 -e MC_CONFIG_DIR=/tmp/mc --entrypoint /bin/sh \
    "$mc_image" -ec '
      mc alias set acceptance http://minio:9000 private_acceptance_minio private_acceptance_minio_secret >/dev/null
      mc cat acceptance/appforge/acceptance/offline-upgrade-marker.txt
    ')
  [[ $database_marker == database-preserved && $object_marker == object-preserved ]] || {
    echo "验收失败: 应用回滚后持久数据不一致" >&2
    exit 1
  }
  rollback_completed=true

  sleep 2
  docker rm -f "$upgrade_probe_container" >/dev/null
  upgrade_probe_container=''
  python3 - "$temporary/upgrade-probe/results" "$temporary/upgrade-probe/metrics.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    statuses = [line.strip() for line in source if line.strip()]
failures = sum(status == "failed" for status in statuses)
maximum = current = 0
for status in statuses:
    if status == "failed":
        current += 1
        maximum = max(maximum, current)
    else:
        current = 0
with open(sys.argv[2], "w", encoding="utf-8") as output:
    json.dump({"requests": len(statuses), "failures": failures, "maxConsecutiveFailures": maximum}, output)
PY
  upgrade_probe_requests=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["requests"])' "$temporary/upgrade-probe/metrics.json")
  upgrade_probe_failures=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["failures"])' "$temporary/upgrade-probe/metrics.json")
  upgrade_probe_max_consecutive_failures=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["maxConsecutiveFailures"])' "$temporary/upgrade-probe/metrics.json")
  ((upgrade_probe_requests > 0)) || { echo "验收失败: 离线升级探针没有结果" >&2; exit 1; }
  ((upgrade_probe_max_consecutive_failures <= upgrade_max_consecutive_failures)) || {
    echo "验收失败: 离线升级最大连续探针失败=${upgrade_probe_max_consecutive_failures}，超过门禁=${upgrade_max_consecutive_failures}" >&2
    exit 1
  }
fi

if [[ -n $report_file ]]; then
  mkdir -p "$(dirname "$report_file")"
  umask 077
  APPFORGE_OFFLINE_ACCEPTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  APPFORGE_OFFLINE_DEPLOYMENT_MODE="$deployment_mode" \
  APPFORGE_OFFLINE_NETWORK_MODE="$offline_mode" \
  APPFORGE_OFFLINE_FORMAL_RELEASE_MEDIA="$formal_release_media" \
  APPFORGE_OFFLINE_FORMAL_RELEASE_PLATFORM="$formal_release_platform" \
  APPFORGE_OFFLINE_MIGRATE_IMAGE="$migrate_image" \
  APPFORGE_OFFLINE_MIGRATE_IMAGE_ID="$migrate_image_id" \
  APPFORGE_OFFLINE_UPGRADE_MODE="$upgrade_mode" \
  APPFORGE_OFFLINE_UPGRADE_FROM="$version" \
  APPFORGE_OFFLINE_UPGRADE_TO="$upgrade_version" \
  APPFORGE_OFFLINE_UPGRADE_COMPLETED="$upgrade_completed" \
  APPFORGE_OFFLINE_ROLLBACK_COMPLETED="$rollback_completed" \
  APPFORGE_OFFLINE_UPGRADE_DURATION="$upgrade_duration_seconds" \
  APPFORGE_OFFLINE_ROLLBACK_DURATION="$rollback_duration_seconds" \
  APPFORGE_OFFLINE_PROBE_REQUESTS="$upgrade_probe_requests" \
  APPFORGE_OFFLINE_PROBE_FAILURES="$upgrade_probe_failures" \
  APPFORGE_OFFLINE_PROBE_MAX_CONSECUTIVE_FAILURES="$upgrade_probe_max_consecutive_failures" \
  APPFORGE_OFFLINE_PROBE_MAX_ALLOWED_FAILURES="$upgrade_max_consecutive_failures" \
  python3 <<'PY' >"$report_file"
import json
import os
import sys

offline = os.environ["APPFORGE_OFFLINE_NETWORK_MODE"] == "1"
formal_release_media = os.environ["APPFORGE_OFFLINE_FORMAL_RELEASE_MEDIA"] == "1"
upgrade_enabled = os.environ["APPFORGE_OFFLINE_UPGRADE_MODE"] == "1"
verified = [
    "schema-migration",
    "fixed-non-root-services",
    "offline-license",
    "bootstrap-admin-login",
    "private-object-storage-bucket",
]
limitations = [
    "local-docker-simulation-not-physical-air-gap",
    "synthetic-temporary-credentials-and-license",
]
if formal_release_media:
    verified.extend(["formal-release-media", "fixed-target-platform", "signed-index-to-platform-digest-map"])
else:
    limitations.append("preloaded-development-images-not-formal-release-media")
upgrade = None
if upgrade_enabled:
    verified.extend(["offline-application-upgrade", "application-rollback", "database-and-object-persistence"])
    limitations.extend([
        "upgrade-tag-aliases-use-identical-development-image-digests",
        "single-node-compose-does-not-promise-zero-downtime",
        "schema-remains-at-113-during-application-upgrade",
    ])
    upgrade = {
        "fromVersion": os.environ["APPFORGE_OFFLINE_UPGRADE_FROM"],
        "toVersion": os.environ["APPFORGE_OFFLINE_UPGRADE_TO"],
        "upgradeCompleted": os.environ["APPFORGE_OFFLINE_UPGRADE_COMPLETED"] == "true",
        "rollbackCompleted": os.environ["APPFORGE_OFFLINE_ROLLBACK_COMPLETED"] == "true",
        "upgradeDurationSeconds": float(os.environ["APPFORGE_OFFLINE_UPGRADE_DURATION"]),
        "rollbackDurationSeconds": float(os.environ["APPFORGE_OFFLINE_ROLLBACK_DURATION"]),
        "databaseMarkerPreserved": True,
        "objectMarkerPreserved": True,
        "healthProbe": {
            "requests": int(os.environ["APPFORGE_OFFLINE_PROBE_REQUESTS"]),
            "failures": int(os.environ["APPFORGE_OFFLINE_PROBE_FAILURES"]),
            "maxConsecutiveFailures": int(os.environ["APPFORGE_OFFLINE_PROBE_MAX_CONSECUTIVE_FAILURES"]),
            "maxAllowedConsecutiveFailures": int(os.environ["APPFORGE_OFFLINE_PROBE_MAX_ALLOWED_FAILURES"]),
        },
    }
else:
    limitations.append("fresh-install-only-not-offline-upgrade")

report = {
    "schemaVersion": 1,
    "acceptedAt": os.environ["APPFORGE_OFFLINE_ACCEPTED_AT"],
    "scope": "synthetic-local-formal-release-media" if formal_release_media else "synthetic-local",
    "acceptanceScript": "deploy/acceptance/v7-private-install.sh",
    "deploymentMode": os.environ["APPFORGE_OFFLINE_DEPLOYMENT_MODE"],
    "migrateImage": os.environ["APPFORGE_OFFLINE_MIGRATE_IMAGE"],
    "migrateImageId": os.environ["APPFORGE_OFFLINE_MIGRATE_IMAGE_ID"],
    "schemaTarget": "20260815_113_v7_air_gapped",
    "schema113ProductionTarget": True,
    "freshVolumes": True,
    "imagesPreloaded": offline,
    "formalReleaseMedia": formal_release_media,
    "formalReleasePlatform": os.environ["APPFORGE_OFFLINE_FORMAL_RELEASE_PLATFORM"] if formal_release_media else None,
    "composePullPolicy": "never",
    "frontendNetworkInternal": offline,
    "backendNetworkInternal": True,
    "internalApiReachable": True,
    "externalHttpsRejected": offline,
    "tlsFrontends": ["admin", "agent"],
    "verified": verified,
    "limitations": limitations,
}
if upgrade is not None:
    report["offlineApplicationUpgrade"] = upgrade
json.dump(report, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
PY
  chmod 0600 "$report_file"
fi

if [[ $upgrade_mode == 1 ]]; then
  echo "通过: 本地模拟断网应用升级 ${version}->${upgrade_version} 并回滚，数据库/对象数据保持；探针请求=${upgrade_probe_requests} 失败=${upgrade_probe_failures} 最大连续失败=${upgrade_probe_max_consecutive_failures}"
elif [[ $offline_mode == 1 ]]; then
  echo "通过: 本地模拟断网全新${deployment_mode}安装；镜像仅预载、双internal网络、内部API可达、外部HTTPS拒绝、迁移、非root、TLS、许可证、登录和对象存储均通过"
else
  echo "通过: 全新${deployment_mode}环境迁移、非root服务、TLS双前端、许可证、启动管理员登录和对象存储初始化"
fi
