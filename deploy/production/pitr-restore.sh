#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 3 ]] || { echo "用法: $0 BASE_BACKUP_DIRECTORY BINLOG_ARCHIVE_DIRECTORY 'YYYY-MM-DD HH:MM:SS'" >&2; exit 1; }
delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
base_backup=$(cd "$1" && pwd)
binlog_archive=$(cd "$2" && pwd)
stop_datetime=$3
[[ $stop_datetime =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}[[:space:]][0-9]{2}:[0-9]{2}:[0-9]{2}$ ]] || {
  echo "截止时间必须是UTC格式 YYYY-MM-DD HH:MM:SS" >&2
  exit 1
}
set -a
# shellcheck disable=SC1091
source "${APPFORGE_ENV_FILE:-$delivery_dir/.env}"
set +a

(cd "$base_backup" && sha256sum -c SHA256SUMS)
(cd "$binlog_archive" && sha256sum -c SHA256SUMS)
# shellcheck disable=SC1091
source "$base_backup/mysql-binlog-position.env"
base_file=${APPFORGE_BINLOG_FILE:-}
base_position=${APPFORGE_BINLOG_POSITION:-}
[[ $base_file =~ ^mysql-bin\.[0-9]{6}$ && $base_position =~ ^[0-9]+$ ]] || {
  echo "基线备份缺少有效的二进制日志坐标" >&2
  exit 1
}
expected_confirmation="$base_backup|$binlog_archive|$stop_datetime"
[[ ${APPFORGE_PITR_CONFIRM:-} == "$expected_confirmation" ]] || {
  echo "指定时间点恢复会覆盖目标环境。请设置 APPFORGE_PITR_CONFIRM='$expected_confirmation'" >&2
  exit 1
}

logs=()
base_found=false
while IFS= read -r log_file; do
  [[ $log_file =~ ^mysql-bin\.[0-9]{6}$ && -f $binlog_archive/$log_file ]] || {
    echo "归档清单包含非法或缺失日志: $log_file" >&2
    exit 1
  }
  if [[ $log_file == "$base_file" ]]; then
    base_found=true
  fi
  [[ $base_found == true ]] && logs+=("$log_file")
done <"$binlog_archive/BINLOGS"
[[ $base_found == true && ${#logs[@]} -gt 0 ]] || { echo "归档不包含基线日志: $base_file" >&2; exit 1; }

APPFORGE_RESTORE_CONFIRM="$base_backup" APPFORGE_RESTORE_SKIP_START=true \
  "$delivery_dir/restore.sh" "$base_backup"

for index in "${!logs[@]}"; do
  log_file=${logs[$index]}
  arguments=(--verify-binlog-checksum --stop-datetime="$stop_datetime")
  if [[ $index -eq 0 ]]; then
    arguments+=(--start-position="$base_position")
  fi
  arguments+=("/binlogs/$log_file")
  docker compose -f "$delivery_dir/docker-compose.yml" run --rm --no-deps \
    --entrypoint /bin/bash -e TZ=UTC -v "$binlog_archive:/binlogs:ro" binlog-tools -c \
    'set -euo pipefail; export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"; mysqlbinlog "$@" | mysql -hmysql -uroot "$MYSQL_DATABASE"' \
    sh "${arguments[@]}"
done

docker compose -f "$delivery_dir/docker-compose.yml" up -d
echo "指定时间点恢复已启动: $stop_datetime UTC；请执行 acceptance/enterprise-delivery.sh 核对业务数据与审计摘要"
