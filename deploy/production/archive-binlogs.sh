#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 2 ]] || { echo "用法: $0 BASE_BACKUP_DIRECTORY OUTPUT_DIRECTORY" >&2; exit 1; }
delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
base_backup=$(cd "$1" && pwd)
output=$2
[[ ! -e $output ]] || { echo "归档目录已存在: $output" >&2; exit 1; }
set -a
# shellcheck disable=SC1091
source "${APPFORGE_ENV_FILE:-$delivery_dir/.env}"
set +a

(cd "$base_backup" && sha256sum -c SHA256SUMS)
# 坐标文件是已校验备份的一部分，只包含受限格式的两个变量。
# shellcheck disable=SC1091
source "$base_backup/mysql-binlog-position.env"
base_file=${APPFORGE_BINLOG_FILE:-}
base_position=${APPFORGE_BINLOG_POSITION:-}
[[ $base_file =~ ^mysql-bin\.[0-9]{6}$ && $base_position =~ ^[0-9]+$ ]] || {
  echo "基线备份缺少有效的二进制日志坐标" >&2
  exit 1
}

umask 077
mkdir -p "$output"
output=$(cd "$output" && pwd)

# 切换到新日志，使归档只读取不会继续增长的文件。
docker compose -f "$delivery_dir/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot -e "FLUSH BINARY LOGS"'
binary_logs=$(docker compose -f "$delivery_dir/docker-compose.yml" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot -N -B -e "SHOW BINARY LOGS"')
active_file=$(printf '%s\n' "$binary_logs" | tail -n 1 | cut -f 1)
[[ $active_file =~ ^mysql-bin\.[0-9]{6}$ ]] || { echo "无法识别当前活动二进制日志" >&2; exit 1; }

started=false
archived=()
while IFS=$'\t' read -r log_file _; do
  [[ $log_file =~ ^mysql-bin\.[0-9]{6}$ ]] || { echo "发现非法二进制日志名称" >&2; exit 1; }
  if [[ $log_file == "$base_file" ]]; then
    started=true
  fi
  [[ $started == true ]] || continue
  [[ $log_file != "$active_file" ]] || continue
  docker compose -f "$delivery_dir/docker-compose.yml" run --rm --no-deps \
    --entrypoint /bin/sh -v "$output:/archive" binlog-tools -c \
    'cd /archive; umask 077; export MYSQL_PWD="$MYSQL_ROOT_PASSWORD"; exec mysqlbinlog --read-from-remote-server --raw --verify-binlog-checksum --host=mysql --user=root --result-file=/archive/ "$1"' \
    sh "$log_file"
  [[ -f $output/$log_file ]] || { echo "归档未生成文件: $log_file" >&2; exit 1; }
  archived+=("$log_file")
done <<<"$binary_logs"

[[ $started == true ]] || { echo "基线日志已不在MySQL保留范围内: $base_file" >&2; exit 1; }
((${#archived[@]} > 0)) || { echo "没有可归档的已关闭二进制日志" >&2; exit 1; }
printf '%s\n' "${archived[@]}" >"$output/BINLOGS"
created_at=$(date -u +%Y%m%dT%H%M%SZ)
printf '{"schemaVersion":1,"createdAt":"%s","baseBinaryLogFile":"%s","baseBinaryLogPosition":%s,"lastArchivedBinaryLogFile":"%s"}\n' \
  "$created_at" "$base_file" "$base_position" "${archived[${#archived[@]}-1]}" >"$output/manifest.json"
(cd "$output" && sha256sum "${archived[@]}" BINLOGS manifest.json >SHA256SUMS)
echo "二进制日志归档完成: $output"
