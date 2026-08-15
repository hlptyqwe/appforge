#!/usr/bin/env bash

set -euo pipefail

[[ $# -eq 2 ]] || { echo "用法: $0 SECRET_NAME.json SECRET_JSON_FILE" >&2; exit 1; }
secret_name=$1
secret_file=$2
[[ $secret_name != */* && $secret_name == *.json ]] || { echo "Secret 名称必须是单个 .json 文件名" >&2; exit 1; }
[[ -f $secret_file && ! -L $secret_file ]] || { echo "Secret 输入必须是普通文件" >&2; exit 1; }
secret_mode=$(stat -c %a "$secret_file" 2>/dev/null || stat -f %Lp "$secret_file")
[[ $secret_mode == 600 ]] || { echo "Secret 输入文件权限必须是 0600" >&2; exit 1; }

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$delivery_dir"
"$delivery_dir/preflight.sh"
APPFORGE_LOCAL_SECRET_NAME="$secret_name" docker compose --profile maintenance run --rm -T secret-import <"$secret_file"
echo "导入完成；控制面签名引用填写 local-file:///$secret_name"
