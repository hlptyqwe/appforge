#!/usr/bin/env bash

set -euo pipefail

[[ $# -ge 1 && $# -le 2 ]] || {
  echo "用法: $0 ABSOLUTE_REPORT.json [SIZE_BYTES]" >&2
  exit 1
}
report_file=$1
size_bytes=${2:-1048576}
[[ $report_file == /* && $report_file == *.json ]] || {
  echo "证据路径必须是绝对 .json 路径" >&2
  exit 1
}
[[ $size_bytes =~ ^[1-9][0-9]*$ ]] || {
  echo "SIZE_BYTES 必须是正整数" >&2
  exit 1
}
((size_bytes >= 1024 && size_bytes <= 67108864)) || {
  echo "SIZE_BYTES 必须在 1024 到 67108864 之间" >&2
  exit 1
}
[[ ! -e $report_file && ! -L $report_file ]] || {
  echo "拒绝覆盖已有证据文件: $report_file" >&2
  exit 1
}

report_dir=$(dirname "$report_file")
[[ -d $report_dir && ! -L $report_dir ]] || {
  echo "证据目录必须已存在且不能是符号链接: $report_dir" >&2
  exit 1
}
report_dir=$(cd "$report_dir" && pwd -P)
report_file="$report_dir/$(basename "$report_file")"
temporary=$(mktemp "$report_dir/.appforge-customer-storage-probe.XXXXXX")
cleanup() {
  rm -f "$temporary"
}
trap cleanup EXIT
chmod 0600 "$temporary"

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$delivery_dir"
"$delivery_dir/preflight.sh"
APPFORGE_CUSTOMER_STORAGE_PROBE_SIZE_BYTES="$size_bytes" \
  docker compose --profile maintenance run --rm -T customer-storage-probe >"$temporary"

[[ -s $temporary ]] || {
  echo "Local Agent 未生成客户对象存储证据" >&2
  exit 1
}
mv "$temporary" "$report_file"
chmod 0600 "$report_file"
trap - EXIT
echo "客户对象存储合成现场探针通过；证据: $report_file"
