#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d)
delivery="$temporary/production"
project="appforge-v7-pitr-$$"
report_file=${APPFORGE_PITR_REPORT_FILE:-}
tools_image="appforge-v7-mysql-binlog-tools:$$"
[[ $project =~ ^appforge-v7-pitr-[0-9]+$ ]]
[[ $tools_image =~ ^appforge-v7-mysql-binlog-tools:[0-9]+$ ]]

now_millis() {
  python3 -c 'import time; print(time.time_ns() // 1_000_000)'
}

cleanup() {
  set +e
  if [[ -f $delivery/docker-compose.yml && -f $delivery/.env ]]; then
    docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" down -v --remove-orphans >/dev/null 2>&1
  fi
  docker image rm "$tools_image" >/dev/null 2>&1
  rm -rf "$temporary"
}
trap cleanup EXIT

mkdir -p "$delivery"
cp "$repo_root/deploy/production/backup.sh" "$delivery/backup.sh"
cp "$repo_root/deploy/production/restore.sh" "$delivery/restore.sh"
cp "$repo_root/deploy/production/archive-binlogs.sh" "$delivery/archive-binlogs.sh"
cp "$repo_root/deploy/production/pitr-restore.sh" "$delivery/pitr-restore.sh"
chmod 0755 "$delivery"/*.sh
docker build --pull=false -f "$repo_root/deploy/docker/mysql-binlog-tools.Dockerfile" -t "$tools_image" "$repo_root" >"$temporary/tools-build.log" 2>&1 || {
  tail -160 "$temporary/tools-build.log" >&2
  exit 1
}
tools_image_digest=$(docker image inspect "$tools_image" --format '{{.Id}}')
tools_version=$(docker run --rm --entrypoint mysqlbinlog "$tools_image" --version)
[[ $tools_image_digest =~ ^sha256:[a-f0-9]{64}$ && $tools_version == mysqlbinlog\ \ Ver\ 8.4.* ]]

cat >"$delivery/docker-compose.yml" <<'YAML'
services:
  mysql:
    image: mysql:8.4
    command: ["--log-bin=mysql-bin", "--server-id=1", "--binlog-format=ROW", "--binlog-row-image=FULL", "--binlog-expire-logs-seconds=604800"]
    environment:
      MYSQL_DATABASE: appforge
      MYSQL_USER: appforge
      MYSQL_PASSWORD: pitr_mysql_password
      MYSQL_ROOT_PASSWORD: pitr_root_password
    volumes: [mysql-data:/var/lib/mysql]
    healthcheck:
      test: ["CMD-SHELL", "MYSQL_PWD=$$MYSQL_ROOT_PASSWORD mysqladmin ping -h127.0.0.1 -uroot --silent"]
      interval: 2s
      timeout: 2s
      retries: 60
  etcd:
    image: quay.io/coreos/etcd:v3.6.12
    command: ["/usr/local/bin/etcd", "--name=default", "--data-dir=/etcd-data", "--listen-client-urls=http://0.0.0.0:2379", "--advertise-client-urls=http://etcd:2379"]
    volumes: [etcd-data:/etcd-data]
    healthcheck:
      test: ["CMD", "/usr/local/bin/etcdctl", "--endpoints=http://127.0.0.1:2379", "endpoint", "health"]
      interval: 2s
      timeout: 2s
      retries: 60
  minio:
    image: minio/minio:RELEASE.2025-04-22T22-12-26Z
    command: ["server", "/data", "--address", ":9000"]
    environment:
      MINIO_ROOT_USER: pitr-access
      MINIO_ROOT_PASSWORD: pitr-secret-password
    volumes: [object-data:/data]
  archive:
    image: mysql:8.4
    profiles: [tools]
    volumes: [object-data:/data, etcd-data:/etcd-data]
  binlog-tools:
    image: ${APPFORGE_BINLOG_TOOLS_IMAGE:?required}
    profiles: [tools]
    user: "65532:65532"
    environment:
      MYSQL_DATABASE: appforge
      MYSQL_ROOT_PASSWORD: pitr_root_password
  api: &dummy
    image: mysql:8.4
    entrypoint: ["/bin/sh", "-c"]
    command: ["while true; do sleep 3600; done"]
  system-rpc: *dummy
  core-rpc: *dummy
  builder-rpc: *dummy
  builder-worker: *dummy
  webhook-worker: *dummy
  billing-worker: *dummy
  enterprise-worker: *dummy
  source-trigger-worker: *dummy
  admin-web: *dummy
  agent-web: *dummy
  minio-init: *dummy
volumes:
  mysql-data:
  etcd-data:
  object-data:
YAML

cat >"$delivery/.env" <<EOF
COMPOSE_PROJECT_NAME=$project
APPFORGE_MYSQL_DATABASE=appforge
APPFORGE_MYSQL_USER=appforge
APPFORGE_MYSQL_PASSWORD=pitr_mysql_password
APPFORGE_MYSQL_ROOT_PASSWORD=pitr_root_password
APPFORGE_VERSION=1.1.0
APPFORGE_BACKUP_DIR=$temporary/backups
APPFORGE_RPO_MINUTES=15
APPFORGE_RTO_MINUTES=120
APPFORGE_BINLOG_TOOLS_IMAGE=$tools_image
EOF
chmod 0600 "$delivery/.env"

compose=(docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml")
"${compose[@]}" up -d --wait mysql etcd minio >"$temporary/start.log" 2>&1 || {
  tail -80 "$temporary/start.log" >&2
  exit 1
}
"${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -e "CREATE TABLE sys_schema_migration (version VARCHAR(64) PRIMARY KEY,description VARCHAR(255) NOT NULL,applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP); INSERT INTO sys_schema_migration(version,description) VALUES (\"20260815_112_v7_customer_storage\",\"PITR合成验收\"); CREATE TABLE pitr_probe(id BIGINT PRIMARY KEY,value VARCHAR(64) NOT NULL,created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP); INSERT INTO pitr_probe(id,value) VALUES (1,\"baseline\");"'
"${compose[@]}" exec -T etcd etcdctl --endpoints=http://127.0.0.1:2379 put /appforge/pitr baseline >/dev/null
"${compose[@]}" exec -T minio sh -c 'mkdir -p /data/appforge && printf "%s" baseline >/data/appforge/pitr.txt'

APPFORGE_ENV_FILE="$delivery/.env" "$delivery/backup.sh" >"$temporary/backup.log" 2>&1 || {
  tail -160 "$temporary/backup.log" >&2
  exit 1
}
base_backup=$(find "$temporary/backups" -mindepth 1 -maxdepth 1 -type d -print -quit)
[[ -n $base_backup && -f $base_backup/mysql-binlog-position.env ]]
(cd "$base_backup" && sha256sum -c SHA256SUMS >/dev/null)
source "$base_backup/mysql-binlog-position.env"
[[ ${APPFORGE_BINLOG_FILE:-} =~ ^mysql-bin\.[0-9]{6}$ && ${APPFORGE_BINLOG_POSITION:-} =~ ^[0-9]+$ ]]
grep -q -- "SOURCE_LOG_FILE='${APPFORGE_BINLOG_FILE}'" "$base_backup/mysql.sql"
grep -q -- "SOURCE_LOG_POS=${APPFORGE_BINLOG_POSITION}" "$base_backup/mysql.sql"

"${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -e "INSERT INTO pitr_probe(id,value) VALUES (2,\"before-cutoff\");"'
sleep 2
stop_datetime=$(date -u '+%Y-%m-%d %H:%M:%S')
sleep 2
"${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -e "INSERT INTO pitr_probe(id,value) VALUES (3,\"after-cutoff\");"'

binlog_archive="$temporary/binlog-archive"
APPFORGE_ENV_FILE="$delivery/.env" "$delivery/archive-binlogs.sh" "$base_backup" "$binlog_archive" >"$temporary/archive.log" 2>&1 || {
  tail -160 "$temporary/archive.log" >&2
  exit 1
}
(cd "$binlog_archive" && sha256sum -c SHA256SUMS >/dev/null)
grep -qx "$APPFORGE_BINLOG_FILE" "$binlog_archive/BINLOGS"

# 显式破坏数据库状态，证明结果来自基线和日志回放而非现存数据。
"${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -e "DROP TABLE pitr_probe;"'
restore_started_ms=$(now_millis)
confirmation="$base_backup|$binlog_archive|$stop_datetime"
APPFORGE_ENV_FILE="$delivery/.env" APPFORGE_PITR_CONFIRM="$confirmation" \
  "$delivery/pitr-restore.sh" "$base_backup" "$binlog_archive" "$stop_datetime" >"$temporary/pitr.log" 2>&1 || {
  tail -240 "$temporary/pitr.log" >&2
  exit 1
}

for _ in $(seq 1 60); do
  if "${compose[@]}" exec -T mysql sh -c \
    'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqladmin -h127.0.0.1 -uroot ping --silent' >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
rows=$("${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -N -B -e "SELECT CONCAT(id,\":\",value) FROM pitr_probe ORDER BY id"')
expected_rows=$'1:baseline\n2:before-cutoff'
[[ $rows == "$expected_rows" ]] || {
  echo "验收失败: PITR结果不符合截止时间，实际为: $rows" >&2
  exit 1
}
[[ $("${compose[@]}" exec -T etcd etcdctl --endpoints=http://127.0.0.1:2379 get /appforge/pitr --print-value-only) == baseline ]]
[[ $("${compose[@]}" exec -T minio cat /data/appforge/pitr.txt) == baseline ]]
verified_ms=$(now_millis)
rto_seconds=$(((verified_ms - restore_started_ms + 999) / 1000))
rto_target_seconds=$((120 * 60))
((rto_seconds <= rto_target_seconds))

"${compose[@]}" down -v --remove-orphans >/dev/null
docker image rm "$tools_image" >/dev/null
containers_left=$(docker ps -aq --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
volumes_left=$(docker volume ls -q --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
networks_left=$(docker network ls -q --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
[[ $containers_left == 0 && $volumes_left == 0 && $networks_left == 0 ]]

if [[ -n $report_file ]]; then
  mkdir -p "$(dirname "$report_file")"
  umask 077
  APPFORGE_PITR_STOP="$stop_datetime" \
    APPFORGE_PITR_BASE_FILE="$APPFORGE_BINLOG_FILE" \
    APPFORGE_PITR_BASE_POSITION="$APPFORGE_BINLOG_POSITION" \
    APPFORGE_PITR_TOOLS_DIGEST="$tools_image_digest" \
    APPFORGE_PITR_TOOLS_VERSION="$tools_version" \
    APPFORGE_PITR_RTO_SECONDS="$rto_seconds" \
    APPFORGE_PITR_RTO_TARGET_SECONDS="$rto_target_seconds" \
    APPFORGE_PITR_CONTAINERS_LEFT="$containers_left" \
    APPFORGE_PITR_VOLUMES_LEFT="$volumes_left" \
    APPFORGE_PITR_NETWORKS_LEFT="$networks_left" \
    python3 -c '
import json, os, sys
integer=lambda key: int(os.environ[key])
json.dump({
  "schemaVersion": 1,
  "scenario": "isolated-synthetic-mysql-pitr",
  "acceptanceScript": "deploy/acceptance/v7-mysql-pitr.sh",
  "stopDatetimeUtc": os.environ["APPFORGE_PITR_STOP"],
  "baseBinaryLogFile": os.environ["APPFORGE_PITR_BASE_FILE"],
  "baseBinaryLogPosition": integer("APPFORGE_PITR_BASE_POSITION"),
  "binlogToolsImageDigest": os.environ["APPFORGE_PITR_TOOLS_DIGEST"],
  "mysqlbinlogVersion": os.environ["APPFORGE_PITR_TOOLS_VERSION"],
  "verifiedRows": ["baseline", "before-cutoff"],
  "excludedRows": ["after-cutoff"],
  "verifiedData": ["synthetic-mysql", "synthetic-etcd", "synthetic-object"],
  "rtoSeconds": integer("APPFORGE_PITR_RTO_SECONDS"),
  "rtoTargetSeconds": integer("APPFORGE_PITR_RTO_TARGET_SECONDS"),
  "residualResources": {
    "containers": integer("APPFORGE_PITR_CONTAINERS_LEFT"),
    "volumes": integer("APPFORGE_PITR_VOLUMES_LEFT"),
    "networks": integer("APPFORGE_PITR_NETWORKS_LEFT")
  },
  "dataPolicy": "synthetic-only",
  "limitations": ["not-customer-target-environment", "not-cross-region-object-replication"]
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
' >"$report_file"
  chmod 0600 "$report_file"
fi

echo "通过: 合成MySQL基线坐标、binlog校验归档、破坏性UTC指定时间点恢复、防止截止时间后事件回放；RTO=${rto_seconds}s/${rto_target_seconds}s，残留资源=0"
