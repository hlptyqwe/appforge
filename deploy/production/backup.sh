#!/usr/bin/env bash

set -euo pipefail

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
set -a
# shellcheck disable=SC1091
source "${APPFORGE_ENV_FILE:-$delivery_dir/.env}"
set +a
backup_root=${APPFORGE_BACKUP_DIR:?required}
stamp=$(date -u +%Y%m%dT%H%M%SZ)
target="$backup_root/$stamp"
[[ ! -e "$target" ]] || { echo "备份目录已存在: $target" >&2; exit 1; }
umask 077
mkdir -p "$target"

# 冻结所有写入方，确保数据库、etcd元数据与对象内容来自同一恢复点。
writers=(api system-rpc core-rpc builder-rpc builder-worker webhook-worker billing-worker enterprise-worker source-trigger-worker)
resume_services() {
  docker compose -f "$delivery_dir/docker-compose.yml" up -d "${writers[@]}" minio >/dev/null
}
trap resume_services EXIT
docker compose -f "$delivery_dir/docker-compose.yml" stop "${writers[@]}" minio >/dev/null

# 写入方冻结后记录与逻辑备份一致的二进制日志起点，供指定时间点恢复使用。
binlog_status=$(docker compose -f "$delivery_dir/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot -N -B -e "SHOW BINARY LOG STATUS"')
IFS=$'\t' read -r binlog_file binlog_position _ <<<"$binlog_status"
[[ $binlog_file =~ ^mysql-bin\.[0-9]{6}$ && $binlog_position =~ ^[0-9]+$ ]] || {
  echo "无法取得有效的MySQL二进制日志坐标" >&2
  exit 1
}
printf 'APPFORGE_BINLOG_FILE=%s\nAPPFORGE_BINLOG_POSITION=%s\n' \
  "$binlog_file" "$binlog_position" >"$target/mysql-binlog-position.env"

docker compose -f "$delivery_dir/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqldump -h127.0.0.1 -uroot --single-transaction --source-data=2 --set-gtid-purged=OFF --routines --events "$MYSQL_DATABASE"' > "$target/mysql.sql"
docker compose -f "$delivery_dir/docker-compose.yml" exec -T etcd etcdctl snapshot save /tmp/etcd.snapshot >/dev/null
docker compose -f "$delivery_dir/docker-compose.yml" cp etcd:/tmp/etcd.snapshot "$target/etcd.snapshot" >/dev/null
docker compose -f "$delivery_dir/docker-compose.yml" run --rm --no-deps --entrypoint /bin/sh \
  -v "$target:/backup" archive -c 'tar -C /data -czf /backup/object-data.tar.gz .'

printf '{"version":"%s","createdAt":"%s","rpoMinutes":%s,"schemaVersion":"%s","binaryLogFile":"%s","binaryLogPosition":%s}\n' \
  "$APPFORGE_VERSION" "$stamp" "${APPFORGE_RPO_MINUTES:-15}" \
  "$(docker compose -f "$delivery_dir/docker-compose.yml" exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_PASSWORD" mysql -h127.0.0.1 -u"$MYSQL_USER" -D"$MYSQL_DATABASE" -N -e "SELECT version FROM sys_schema_migration ORDER BY CAST(SUBSTRING_INDEX(version,\"_\",1) AS UNSIGNED) DESC,CAST(SUBSTRING_INDEX(SUBSTRING_INDEX(version,\"_\",2),\"_\",-1) AS UNSIGNED) DESC,applied_at DESC LIMIT 1"')" \
  "$binlog_file" "$binlog_position" \
  > "$target/manifest.json"
(cd "$target" && sha256sum mysql.sql mysql-binlog-position.env etcd.snapshot object-data.tar.gz manifest.json >SHA256SUMS)
resume_services
trap - EXIT
echo "备份完成: $target"
