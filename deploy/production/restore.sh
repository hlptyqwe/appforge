#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 1 ]] || { echo "用法: $0 BACKUP_DIRECTORY" >&2; exit 1; }
delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
backup_dir=$(cd "$1" && pwd)
set -a
# shellcheck disable=SC1091
source "${APPFORGE_ENV_FILE:-$delivery_dir/.env}"
set +a

(cd "$backup_dir" && sha256sum -c SHA256SUMS)
[[ ${APPFORGE_RESTORE_CONFIRM:-} == "$backup_dir" ]] || {
  echo "恢复会覆盖目标环境。请设置 APPFORGE_RESTORE_CONFIRM=$backup_dir" >&2
  exit 1
}

services=(api system-rpc core-rpc builder-rpc builder-worker webhook-worker billing-worker enterprise-worker source-trigger-worker admin-web agent-web etcd minio minio-init)
docker compose -f "$delivery_dir/docker-compose.yml" stop "${services[@]}"

# 目标数据库使用root重建，避免把备份覆盖到残留表上形成混合状态。
docker compose -f "$delivery_dir/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot -e "DROP DATABASE IF EXISTS \`$MYSQL_DATABASE\`; CREATE DATABASE \`$MYSQL_DATABASE\` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci; GRANT ALL PRIVILEGES ON \`$MYSQL_DATABASE\`.* TO '\''$MYSQL_USER'\''@'\''%'\'';"'
docker compose -f "$delivery_dir/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE"' < "$backup_dir/mysql.sql"

# 使用专用工具清空正式数据卷，再以etcdutl固定入口恢复快照。
docker compose -f "$delivery_dir/docker-compose.yml" run --rm --no-deps --entrypoint /bin/sh \
  archive -c 'find /etcd-data -mindepth 1 -delete'
docker compose -f "$delivery_dir/docker-compose.yml" run --rm --no-deps --entrypoint /usr/local/bin/etcdutl \
  -v "$backup_dir:/backup:ro" etcd snapshot restore /backup/etcd.snapshot --data-dir=/etcd-data
docker compose -f "$delivery_dir/docker-compose.yml" run --rm --no-deps --entrypoint /bin/sh \
  -v "$backup_dir:/backup:ro" archive -c \
  'find /data -mindepth 1 -delete && tar -C /data -xzf /backup/object-data.tar.gz'
docker compose -f "$delivery_dir/docker-compose.yml" up -d
echo "恢复已启动，请执行 acceptance/enterprise-delivery.sh 核对租户、任务、对象引用和审计摘要"
