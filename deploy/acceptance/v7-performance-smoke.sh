#!/usr/bin/env bash

set -euo pipefail

api_url=${APPFORGE_V7_API_URL:-http://127.0.0.1:8888}
admin_username=${APPFORGE_V7_ADMIN_USERNAME:-appforge}
admin_password=${APPFORGE_V7_ADMIN_PASSWORD:-AppForge@123}
health_requests=${APPFORGE_PERF_HEALTH_REQUESTS:-500}
health_concurrency=${APPFORGE_PERF_HEALTH_CONCURRENCY:-32}
health_p95_limit=${APPFORGE_PERF_HEALTH_P95_SECONDS:-0.250}
deployment_requests=${APPFORGE_PERF_DEPLOYMENT_REQUESTS:-150}
deployment_concurrency=${APPFORGE_PERF_DEPLOYMENT_CONCURRENCY:-12}
deployment_p95_limit=${APPFORGE_PERF_DEPLOYMENT_P95_SECONDS:-1.000}
temporary=$(mktemp -d /tmp/appforge-v7-performance.XXXXXX)
trap 'rm -rf "$temporary"' EXIT

for command_name in curl python3 awk sort sed seq xargs; do
  command -v "$command_name" >/dev/null || { echo "验收失败: 缺少命令 $command_name" >&2; exit 1; }
done

login_response=$(curl -fsS -H 'Content-Type: application/json' \
  -d "{\"username\":\"${admin_username}\",\"password\":\"${admin_password}\"}" \
  "$api_url/admin/system/auth/login")
admin_token=$(printf '%s' "$login_response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')

printf '%s\n' \
  'silent' \
  'show-error' \
  'output = "/dev/null"' \
  'write-out = "%{http_code} %{time_total}\n"' \
  'connect-timeout = 2' \
  'max-time = 5' >"$temporary/health.curlrc"
printf '%s\n' \
  'silent' \
  'show-error' \
  'output = "/dev/null"' \
  'write-out = "%{http_code} %{time_total}\n"' \
  'connect-timeout = 2' \
  'max-time = 5' \
  "header = \"Authorization: Bearer $admin_token\"" >"$temporary/deployment.curlrc"
chmod 0600 "$temporary/health.curlrc" "$temporary/deployment.curlrc"
unset admin_token login_response

run_probe() {
  local label=$1 url=$2 request_count=$3 concurrency=$4 p95_limit=$5 config_file=$6
  local result_file="$temporary/${label}.results"
  local sorted_file="$temporary/${label}.sorted"

  curl --config "$config_file" "$url" >/dev/null
  seq "$request_count" | xargs -P "$concurrency" -I '{}' \
    curl --config "$config_file" "$url" >"$result_file"

  local actual_count failed_count p95_rank p95 average
  actual_count=$(awk 'NF==2 {count++} END {print count+0}' "$result_file")
  failed_count=$(awk '$1 != 200 {count++} END {print count+0}' "$result_file")
  [[ $actual_count == "$request_count" ]] || {
    echo "验收失败: $label 仅产生 $actual_count/$request_count 条完整结果" >&2
    exit 1
  }
  [[ $failed_count == 0 ]] || { echo "验收失败: $label 有 $failed_count 个非 200 响应" >&2; exit 1; }

  awk '{print $2}' "$result_file" | sort -n >"$sorted_file"
  p95_rank=$(( (request_count * 95 + 99) / 100 ))
  p95=$(sed -n "${p95_rank}p" "$sorted_file")
  average=$(awk '{sum+=$1} END {printf "%.6f", sum/NR}' "$sorted_file")
  awk -v actual="$p95" -v limit="$p95_limit" 'BEGIN {exit !(actual <= limit)}' || {
    echo "验收失败: $label P95=${p95}s 超过门禁 ${p95_limit}s" >&2
    exit 1
  }
  echo "通过: $label 请求=${request_count} 并发=${concurrency} 错误=0 平均=${average}s P95=${p95}s/门禁${p95_limit}s"
}

run_probe healthz "$api_url/healthz" "$health_requests" "$health_concurrency" "$health_p95_limit" "$temporary/health.curlrc"
run_probe deployment "$api_url/admin/core/enterprise/deployment" "$deployment_requests" "$deployment_concurrency" \
  "$deployment_p95_limit" "$temporary/deployment.curlrc"

echo "通过: V7 本地并发性能冒烟；该结果不是客户容量、峰值或长稳承诺"
