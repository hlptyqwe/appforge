#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d)
delivery="$temporary/production"
project="appforge-v7-recovery-$$"
report_file=${APPFORGE_DR_REPORT_FILE:-}
[[ $project =~ ^appforge-v7-recovery-[0-9]+$ ]]

now_millis() {
  python3 -c 'import time; print(time.time_ns() // 1_000_000)'
}

cleanup() {
  set +e
  if [[ -f $delivery/docker-compose.yml && -f $delivery/.env ]]; then
    docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" down -v --remove-orphans >/dev/null 2>&1
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT

mkdir -p "$delivery"
cp "$repo_root/deploy/production/backup.sh" "$delivery/backup.sh"
cp "$repo_root/deploy/production/restore.sh" "$delivery/restore.sh"

cat >"$delivery/docker-compose.yml" <<'YAML'
services:
  mysql:
    image: mysql:8.4
    environment:
      MYSQL_DATABASE: appforge
      MYSQL_USER: appforge
      MYSQL_PASSWORD: recovery_mysql_password
      MYSQL_ROOT_PASSWORD: recovery_root_password
    volumes: [mysql-data:/var/lib/mysql]
    healthcheck:
      test: ["CMD-SHELL", "MYSQL_PWD=$$MYSQL_ROOT_PASSWORD mysqladmin ping -uroot --silent"]
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
      MINIO_ROOT_USER: recovery-access
      MINIO_ROOT_PASSWORD: recovery-secret-password
    volumes: [object-data:/data]
  archive:
    image: mysql:8.4
    profiles: [tools]
    volumes: [object-data:/data, etcd-data:/etcd-data]
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
volumes:
  mysql-data:
  etcd-data:
  object-data:
YAML

cat >"$delivery/.env" <<EOF
COMPOSE_PROJECT_NAME=$project
APPFORGE_MYSQL_DATABASE=appforge
APPFORGE_MYSQL_USER=appforge
APPFORGE_MYSQL_PASSWORD=recovery_mysql_password
APPFORGE_MYSQL_ROOT_PASSWORD=recovery_root_password
APPFORGE_VERSION=1.1.0
APPFORGE_BACKUP_DIR=$temporary/backups
APPFORGE_RPO_MINUTES=15
APPFORGE_RTO_MINUTES=120
EOF
chmod 0600 "$delivery/.env"

docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" up -d --wait mysql etcd minio >"$temporary/start.log" 2>&1 || {
  tail -80 "$temporary/start.log" >&2; exit 1;
}
for _ in $(seq 1 30); do
  if docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T mysql sh -c \
    'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqladmin -h127.0.0.1 -uroot ping --silent' >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqladmin -h127.0.0.1 -uroot ping --silent' >/dev/null
docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -e "CREATE TABLE sys_schema_migration (version VARCHAR(64) PRIMARY KEY,description VARCHAR(255) NOT NULL,applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP); INSERT INTO sys_schema_migration(version,description) VALUES (\"recovery-v1\",\"恢复验收\"); CREATE TABLE recovery_probe(id BIGINT PRIMARY KEY,value VARCHAR(64) NOT NULL,created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)); INSERT INTO recovery_probe(id,value) VALUES (1,\"mysql-preserved\");"'
recovery_marker_ms=$(docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -N -B -e "SELECT CAST(UNIX_TIMESTAMP(created_at)*1000 AS UNSIGNED) FROM recovery_probe WHERE id=1"')
docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T etcd \
  etcdctl --endpoints=http://127.0.0.1:2379 put /appforge/recovery etcd-preserved >/dev/null
docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T minio sh -c \
  'mkdir -p /data/appforge && printf "%s" object-preserved >/data/appforge/recovery.txt'

APPFORGE_ENV_FILE="$delivery/.env" "$delivery/backup.sh" >"$temporary/backup.log" 2>&1 || {
  tail -120 "$temporary/backup.log" >&2; exit 1;
}
backup_completed_ms=$(now_millis)
backup_path=$(find "$temporary/backups" -mindepth 1 -maxdepth 1 -type d -print -quit)
[[ -n $backup_path && -f $backup_path/SHA256SUMS ]]
rpo_seconds=$(((backup_completed_ms - recovery_marker_ms + 999) / 1000))
rpo_target_seconds=$((${APPFORGE_RPO_MINUTES:-15} * 60))
((rpo_seconds <= rpo_target_seconds)) || {
  echo "验收失败: 隔离恢复点间隔 ${rpo_seconds}s 超过目标 ${rpo_target_seconds}s" >&2; exit 1;
}

rto_started_ms=$(now_millis)
docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -e "DELETE FROM recovery_probe"'
docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T etcd \
  etcdctl --endpoints=http://127.0.0.1:2379 del /appforge/recovery >/dev/null
docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T minio \
  rm -f /data/appforge/recovery.txt

APPFORGE_ENV_FILE="$delivery/.env" APPFORGE_RESTORE_CONFIRM="$backup_path" "$delivery/restore.sh" "$backup_path" >"$temporary/restore.log" 2>&1 || {
  tail -160 "$temporary/restore.log" >&2; exit 1;
}
docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" up -d --wait mysql etcd minio >"$temporary/restart.log" 2>&1 || {
  tail -80 "$temporary/restart.log" >&2; exit 1;
}

mysql_value=$(docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -N -B -e "SELECT value FROM recovery_probe WHERE id=1"')
etcd_value=$(docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T etcd \
  etcdctl --endpoints=http://127.0.0.1:2379 get /appforge/recovery --print-value-only)
object_value=$(docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" exec -T minio \
  cat /data/appforge/recovery.txt)
[[ $mysql_value == mysql-preserved && $etcd_value == etcd-preserved && $object_value == object-preserved ]] || {
  echo "验收失败: 恢复数据不一致" >&2; exit 1;
}
rto_completed_ms=$(now_millis)
rto_seconds=$(((rto_completed_ms - rto_started_ms + 999) / 1000))
rto_target_seconds=$((${APPFORGE_RTO_MINUTES:-120} * 60))
((rto_seconds <= rto_target_seconds)) || {
  echo "验收失败: 隔离恢复耗时 ${rto_seconds}s 超过目标 ${rto_target_seconds}s" >&2; exit 1;
}

if [[ -n $report_file ]]; then
  mkdir -p "$(dirname "$report_file")"
  umask 077
  APPFORGE_DR_SCENARIO=isolated-synthetic \
    APPFORGE_DR_MARKER_MS=$recovery_marker_ms \
    APPFORGE_DR_BACKUP_MS=$backup_completed_ms \
    APPFORGE_DR_RESTORE_START_MS=$rto_started_ms \
    APPFORGE_DR_VERIFIED_MS=$rto_completed_ms \
    APPFORGE_DR_RPO_SECONDS=$rpo_seconds \
    APPFORGE_DR_RPO_TARGET_SECONDS=$rpo_target_seconds \
    APPFORGE_DR_RTO_SECONDS=$rto_seconds \
    APPFORGE_DR_RTO_TARGET_SECONDS=$rto_target_seconds \
    python3 -c '
import json,os,sys
integer=lambda key: int(os.environ[key])
json.dump({
  "schemaVersion": 1,
  "scenario": os.environ["APPFORGE_DR_SCENARIO"],
  "acceptanceScript": "deploy/acceptance/v7-backup-restore.sh",
  "recoveryPointMarkerAt": integer("APPFORGE_DR_MARKER_MS"),
  "backupCompletedAt": integer("APPFORGE_DR_BACKUP_MS"),
  "restoreStartedAt": integer("APPFORGE_DR_RESTORE_START_MS"),
  "recoveryVerifiedAt": integer("APPFORGE_DR_VERIFIED_MS"),
  "rpoSeconds": integer("APPFORGE_DR_RPO_SECONDS"),
  "rpoTargetSeconds": integer("APPFORGE_DR_RPO_TARGET_SECONDS"),
  "rtoSeconds": integer("APPFORGE_DR_RTO_SECONDS"),
  "rtoTargetSeconds": integer("APPFORGE_DR_RTO_TARGET_SECONDS"),
  "verifiedData": ["synthetic-mysql", "synthetic-etcd", "synthetic-object"],
  "limitations": ["not-full-business-dataset", "not-customer-target-environment"]
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
' >"$report_file"
  chmod 0600 "$report_file"
fi
echo "通过: 独立环境MySQL、etcd、对象数据一致性备份，破坏后恢复与校验；RPO=${rpo_seconds}s/${rpo_target_seconds}s，RTO=${rto_seconds}s/${rto_target_seconds}s"
