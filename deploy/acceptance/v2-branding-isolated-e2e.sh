#!/usr/bin/env bash

set -euo pipefail

# 在唯一命名、临时卷的 Compose 栈中执行 V2 公共 API -> 真实 Worker 负向 E2E。
# 该脚本复用当前开发镜像，但不连接 appforge-dev 网络、数据库、Redis、etcd 或 MinIO。
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d)
project="appforge-v2-isolated-$(date +%s)-$$"
compose_file="$temporary/docker-compose.yml"
override_file="$temporary/docker-compose.override.yml"
etcd_dir="$temporary/etcd"
seed_file="$temporary/30-seed.sql"
evidence=${APPFORGE_V2_NEGATIVE_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v2-branding-negative-e2e-20260815.json}
stack_started=false

[[ $project =~ ^appforge-v2-isolated-[0-9]+-[0-9]+$ ]]

compose() {
  docker compose -p "$project" --project-directory "$repo_root/deploy" \
    -f "$compose_file" -f "$override_file" "$@"
}

cleanup() {
  local exit_code=$?
  set +e
  if [[ $stack_started == true && -f $compose_file && -f $override_file ]]; then
    if ((exit_code != 0)); then
      echo "V2 隔离 E2E 失败，输出临时栈状态和迁移/API/Core/Builder日志后销毁：" >&2
      compose ps -a >&2
      compose logs --no-color mysql-migrate etcd-init system-rpc core-rpc builder-rpc builder-worker-1 admin-api >&2
    fi
    compose down -v --remove-orphans >/dev/null 2>&1
  fi
  [[ -n $temporary && $temporary == /tmp/* || $temporary == /private/var/folders/* || $temporary == /var/folders/* ]] && rm -rf "$temporary"
}
trap cleanup EXIT

for command in awk cp date docker grep mktemp python3 sed; do
  command -v "$command" >/dev/null || { echo "缺少命令: $command" >&2; exit 1; }
done

required_images=(
  appforge-dev-etcd-init:latest
  appforge-dev-system-rpc:latest
  appforge-dev-core-rpc:latest
  appforge-dev-builder-rpc:latest
  appforge-dev-builder-worker-1:latest
  appforge-dev-admin-api:latest
  appforge-local-agent:dev
  mysql:8.4
  redis:7.4-alpine
  quay.io/coreos/etcd:v3.6.12
  minio/minio:RELEASE.2025-04-22T22-12-26Z
  minio/mc:RELEASE.2025-04-16T18-13-26Z
)
for image in "${required_images[@]}"; do
  docker image inspect "$image" >/dev/null || { echo "缺少隔离 E2E 所需本地镜像: $image" >&2; exit 1; }
done

mkdir -p "$etcd_dir"
chmod 0700 "$temporary" "$etcd_dir"
cp "$repo_root/deploy/etcd/"*.yaml "$etcd_dir/"
cp "$repo_root/deploy/etcd/init-etcd.sh" "$etcd_dir/init-etcd.sh"

read -r mysql_port redis_port etcd_port minio_port minio_console_port system_port core_port builder_port api_port <<<"$(python3 - <<'PY'
import socket

sockets=[]
ports=[]
try:
    for _ in range(9):
        sock=socket.socket()
        sock.bind(('127.0.0.1', 0))
        sockets.append(sock)
        ports.append(sock.getsockname()[1])
    print(*ports)
finally:
    for sock in sockets:
        sock.close()
PY
)"
for port in "$mysql_port" "$redis_port" "$etcd_port" "$minio_port" "$minio_console_port" "$system_port" "$core_port" "$builder_port" "$api_port"; do
  [[ $port =~ ^[1-9][0-9]{3,4}$ ]]
done

sed "s#BucketUrl: http://localhost:9000/appforge#BucketUrl: http://127.0.0.1:${minio_port}/appforge#" \
  "$etcd_dir/builder.yaml" >"$etcd_dir/builder.yaml.tmp"
mv "$etcd_dir/builder.yaml.tmp" "$etcd_dir/builder.yaml"
grep -F "BucketUrl: http://127.0.0.1:${minio_port}/appforge" "$etcd_dir/builder.yaml" >/dev/null
sed "s#http://localhost:9000/appforge#http://127.0.0.1:${minio_port}/appforge#g" \
  "$repo_root/deploy/mysql/init/30-seed.sql" >"$seed_file"
grep -F "'bucket_url', 'http://127.0.0.1:${minio_port}/appforge'" "$seed_file" >/dev/null
! grep -F 'http://localhost:9000/appforge' "$seed_file" >/dev/null

awk '
  NR == 1 && $0 == "name: appforge-dev" { next }
  /^[[:space:]]+container_name:/ { next }
  $0 == "    name: appforge-dev" { next }
  { print }
' "$repo_root/deploy/docker-compose.dev.yml" >"$compose_file"

cat >"$override_file" <<EOF
services:
  mysql:
    volumes:
      - mysql-data:/var/lib/mysql
      - $repo_root/services/system/system.sql:/docker-entrypoint-initdb.d/10-system.sql:ro
      - $repo_root/services/core/core.sql:/docker-entrypoint-initdb.d/20-core.sql:ro
      - $seed_file:/docker-entrypoint-initdb.d/30-seed.sql:ro
  mysql-migrate:
    volumes:
      - $repo_root/deploy/mysql/migrations:/migrations:ro
  etcd-init:
    image: appforge-dev-etcd-init:latest
    volumes:
      - $etcd_dir:/config:ro
      - $etcd_dir/init-etcd.sh:/init/init-etcd.sh:ro
  system-rpc:
    image: appforge-dev-system-rpc:latest
  core-rpc:
    image: appforge-dev-core-rpc:latest
  builder-rpc:
    image: appforge-dev-builder-rpc:latest
  builder-worker-1:
    image: appforge-dev-builder-worker-1:latest
  admin-api:
    image: appforge-dev-admin-api:latest
EOF

export APPFORGE_MYSQL_PORT=$mysql_port
export APPFORGE_REDIS_PORT=$redis_port
export APPFORGE_ETCD_PORT=$etcd_port
export APPFORGE_MINIO_PORT=$minio_port
export APPFORGE_MINIO_CONSOLE_PORT=$minio_console_port
export APPFORGE_SYSTEM_RPC_PORT=$system_port
export APPFORGE_CORE_RPC_PORT=$core_port
export APPFORGE_BUILDER_RPC_PORT=$builder_port
export APPFORGE_API_PORT=$api_port

rendered=$(compose config)
! grep -F 'container_name:' <<<"$rendered" >/dev/null
! grep -F 'name: appforge-dev' <<<"$rendered" >/dev/null
grep -F "name: ${project}_appforge" <<<"$rendered" >/dev/null
grep -F "source: $repo_root/services/system/system.sql" <<<"$rendered" >/dev/null
grep -F "source: $repo_root/services/core/core.sql" <<<"$rendered" >/dev/null
grep -F "source: $seed_file" <<<"$rendered" >/dev/null
grep -F "source: $repo_root/deploy/mysql/migrations" <<<"$rendered" >/dev/null

stack_started=true
compose up -d --no-build \
  mysql mysql-migrate redis etcd etcd-init minio minio-init \
  system-rpc core-rpc builder-rpc builder-worker-1 admin-api

for _ in $(seq 1 120); do
  if curl -fsS "http://127.0.0.1:${api_port}/admin/system/core" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${api_port}/admin/system/core" >/dev/null

mysql_container="$project-mysql-1"
core_container="$project-core-rpc-1"
builder_rpc_container="$project-builder-rpc-1"
api_container="$project-admin-api-1"
minio_container="$project-minio-1"
network="${project}_appforge"

for container in "$mysql_container" "$core_container" "$builder_rpc_container" "$api_container" "$minio_container"; do
  [[ $(docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' "$container") == "$project" ]]
done
[[ $(docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' appforge-mysql 2>/dev/null || true) != "$project" ]]

APPFORGE_V2_ALLOW_WRITE_E2E=true \
APPFORGE_V2_API_URL="http://127.0.0.1:${api_port}" \
APPFORGE_V2_MYSQL_CONTAINER="$mysql_container" \
APPFORGE_DEV_NETWORK="$network" \
APPFORGE_V2_COMPOSE_PROJECT="$project" \
APPFORGE_V2_ISOLATED_PROJECT="$project" \
APPFORGE_V2_CORE_CONTAINER="$core_container" \
APPFORGE_V2_BUILDER_RPC_CONTAINER="$builder_rpc_container" \
APPFORGE_V2_API_CONTAINER="$api_container" \
APPFORGE_V2_MINIO_CONTAINER="$minio_container" \
APPFORGE_V2_EXPECT_UPLOAD_ORIGIN="http://127.0.0.1:${minio_port}" \
APPFORGE_V2_NEGATIVE_EVIDENCE_PATH="$evidence" \
  "$repo_root/deploy/acceptance/v2-branding-negative-e2e.sh"

compose down -v --remove-orphans
stack_started=false

remaining_containers=$(docker ps -aq --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
remaining_volumes=$(docker volume ls -q --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
remaining_networks=$(docker network ls -q --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
[[ $remaining_containers == 0 && $remaining_volumes == 0 && $remaining_networks == 0 ]]

EVIDENCE_PATH="$evidence" PROJECT="$project" python3 - <<'PY'
import json,os,pathlib

path=pathlib.Path(os.environ['EVIDENCE_PATH'])
payload=json.loads(path.read_text(encoding='utf-8'))
assert payload['mode'] == 'isolated-compose-synthetic-v2-negative-e2e'
assert payload['inputs']['temporaryIsolatedEnvironment'] is True
assert payload['inputs']['sharedDevelopmentDatabaseMutated'] is False
payload['isolatedComposeProject'] = os.environ['PROJECT']
payload['checks']['isolatedComposeDestroyed'] = 'passed'
payload['cleanup']['isolatedContainersRemaining'] = 0
payload['cleanup']['isolatedVolumesRemaining'] = 0
payload['cleanup']['isolatedNetworksRemaining'] = 0
path.write_text(json.dumps(payload,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
PY
chmod 0600 "$evidence"

echo "V2 隔离负向 E2E 通过：公共 API、真实 Worker、INCOMPATIBLE、零正式任务和完整临时栈销毁均已验证。"
echo "证据: $evidence"
