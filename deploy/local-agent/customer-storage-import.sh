#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 3 ]] || { echo "用法: $0 APP_ID OBJECT_TYPE INPUT_FILE" >&2; exit 1; }
app_id=$1
object_type=$2
input_file=$3
[[ $app_id =~ ^[1-9][0-9]*$ ]] || { echo "APP_ID 必须是正整数" >&2; exit 1; }
case "$object_type" in
  source-apk|keystore|brand-logo|brand-splash|template-file) ;;
  *) echo "OBJECT_TYPE 不受支持" >&2; exit 1 ;;
esac
[[ -f $input_file && ! -L $input_file ]] || { echo "输入必须是非符号链接普通文件" >&2; exit 1; }
input_file=$(cd "$(dirname "$input_file")" && pwd)/$(basename "$input_file")

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$delivery_dir"
"$delivery_dir/preflight.sh"
APPFORGE_CUSTOMER_APP_ID="$app_id" APPFORGE_CUSTOMER_OBJECT_TYPE="$object_type" \
APPFORGE_CUSTOMER_INPUT_FILE="$input_file" \
  docker compose --profile maintenance run --rm -T customer-storage-import
