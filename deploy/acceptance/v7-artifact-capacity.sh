#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
api_url=${APPFORGE_CAPACITY_API_URL:-http://127.0.0.1:8888}
docker_network=${APPFORGE_CAPACITY_DOCKER_NETWORK:-appforge-dev}
agent_image=${APPFORGE_CAPACITY_AGENT_IMAGE:-appforge-local-agent:dev}
mc_image=${APPFORGE_CAPACITY_MC_IMAGE:-minio/mc:RELEASE.2025-04-16T18-13-26Z}
minio_endpoint=${APPFORGE_CAPACITY_MINIO_ENDPOINT:-http://minio:9000}
minio_access_key=${APPFORGE_CAPACITY_MINIO_ACCESS_KEY:-appforge}
minio_secret_key=${APPFORGE_CAPACITY_MINIO_SECRET_KEY:-appforge_dev_minio}
minio_bucket=${APPFORGE_CAPACITY_MINIO_BUCKET:-appforge}
minio_container=${APPFORGE_CAPACITY_MINIO_CONTAINER:-appforge-minio}
build_concurrency=${APPFORGE_CAPACITY_BUILD_CONCURRENCY:-4}
object_copies=${APPFORGE_CAPACITY_OBJECT_COPIES:-4}
object_mib=${APPFORGE_CAPACITY_OBJECT_MIB:-32}
object_min_mibps=${APPFORGE_CAPACITY_OBJECT_MIN_MIBPS:-1}
soak_seconds=${APPFORGE_CAPACITY_SOAK_SECONDS:-120}
soak_concurrency=${APPFORGE_CAPACITY_SOAK_CONCURRENCY:-8}
soak_p99_limit=${APPFORGE_CAPACITY_SOAK_P99_SECONDS:-2}
report_file=${APPFORGE_CAPACITY_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-artifact-capacity-20260815.json}
temporary=$(mktemp -d /tmp/appforge-v7-capacity.XXXXXX)
suffix="$(date +%s)-$$"
object_prefix="acceptance/v7-capacity-$suffix"
object_prefix_active=true

require_positive_integer() {
  local name=$1 value=$2
  [[ $value =~ ^[1-9][0-9]*$ ]] || {
    echo "验收失败: $name 必须是正整数" >&2
    exit 1
  }
}

for pair in \
  "APPFORGE_CAPACITY_BUILD_CONCURRENCY:$build_concurrency" \
  "APPFORGE_CAPACITY_OBJECT_COPIES:$object_copies" \
  "APPFORGE_CAPACITY_OBJECT_MIB:$object_mib" \
  "APPFORGE_CAPACITY_SOAK_SECONDS:$soak_seconds" \
  "APPFORGE_CAPACITY_SOAK_CONCURRENCY:$soak_concurrency"; do
  require_positive_integer "${pair%%:*}" "${pair#*:}"
done
[[ $minio_bucket =~ ^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$ ]] || {
  echo "验收失败: MinIO Bucket 名称不安全" >&2
  exit 1
}
[[ $object_prefix =~ ^acceptance/v7-capacity-[0-9]+-[0-9]+$ ]]

cleanup_object_prefix() {
  [[ $object_prefix_active == true ]] || return 0
  docker run --rm --network "$docker_network" \
    --entrypoint /bin/sh \
    -e MC_ENDPOINT="$minio_endpoint" \
    -e MC_ACCESS_KEY="$minio_access_key" \
    -e MC_SECRET_KEY="$minio_secret_key" \
    -e MC_BUCKET="$minio_bucket" \
    -e MC_PREFIX="$object_prefix" \
    "$mc_image" -ec '
      mc alias set local "$MC_ENDPOINT" "$MC_ACCESS_KEY" "$MC_SECRET_KEY" >/dev/null
      mc rm --recursive --force "local/$MC_BUCKET/$MC_PREFIX/" >/dev/null 2>&1
    ' >/dev/null 2>&1
}

cleanup() {
  set +e
  cleanup_object_prefix >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT

for command_name in awk curl dd docker python3; do
  command -v "$command_name" >/dev/null || {
    echo "验收失败: 缺少命令 $command_name" >&2
    exit 1
  }
done
docker network inspect "$docker_network" >/dev/null
[[ $(docker inspect -f '{{.State.Running}}' "$minio_container" 2>/dev/null) == true ]] || {
  echo "验收失败: MinIO 容器 $minio_container 未运行" >&2
  exit 1
}
docker image inspect "$agent_image" >/dev/null || {
  echo "验收失败: 缺少 Local Agent 镜像 $agent_image" >&2
  exit 1
}
docker image inspect "$mc_image" >/dev/null || {
  echo "验收失败: 缺少 MinIO Client 镜像 $mc_image" >&2
  exit 1
}
curl -fsS --connect-timeout 2 --max-time 5 "$api_url/healthz" >/dev/null

now_ns() {
  python3 -c 'import time; print(time.time_ns())'
}

duration_seconds() {
  python3 - "$1" "$2" <<'PY'
import sys
started, completed = map(int, sys.argv[1:])
print(f"{(completed - started) / 1_000_000_000:.6f}")
PY
}

rate_per_second() {
  python3 - "$1" "$2" <<'PY'
import sys
amount, duration = map(float, sys.argv[1:])
print(f"{amount / duration:.3f}")
PY
}

sha256_file() {
  if command -v sha256sum >/dev/null; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi
  command -v shasum >/dev/null || {
    echo "验收失败: 缺少 sha256sum 或 shasum" >&2
    exit 1
  }
  shasum -a 256 "$1" | awk '{print $1}'
}

echo "开始: 合成 APK 并发构建，任务数=$build_concurrency"
build_started_ns=$(now_ns)
build_pids=()
for ((index = 1; index <= build_concurrency; index++)); do
  APPFORGE_LOCAL_AGENT_IMAGE="$agent_image" \
    "$repo_root/deploy/acceptance/local-agent-executor.sh" \
    >"$temporary/build-$index.log" 2>&1 &
  build_pids+=("$!")
done
build_failures=0
for ((index = 1; index <= build_concurrency; index++)); do
  if ! wait "${build_pids[$((index - 1))]}"; then
    build_failures=$((build_failures + 1))
    echo "并发构建 $index 失败:" >&2
    tail -80 "$temporary/build-$index.log" >&2
  fi
done
build_completed_ns=$(now_ns)
((build_failures == 0)) || {
  echo "验收失败: $build_failures/$build_concurrency 个合成 APK 构建失败" >&2
  exit 1
}
build_duration=$(duration_seconds "$build_started_ns" "$build_completed_ns")
builds_per_minute=$(rate_per_second "$((build_concurrency * 60))" "$build_duration")

echo "开始: 本地 MinIO 并发上传/下载，文件=${object_mib}MiB 副本=$object_copies"
dd if=/dev/urandom of="$temporary/object.bin" bs=1048576 count="$object_mib" 2>/dev/null
object_sha256=$(sha256_file "$temporary/object.bin")
mkdir -p "$temporary/downloads"
chmod 0777 "$temporary/downloads"

upload_started_ns=$(now_ns)
upload_pids=()
for ((index = 1; index <= object_copies; index++)); do
  docker run --rm --network "$docker_network" \
    --entrypoint /bin/sh \
    -v "$temporary:/work:ro" \
    -e MC_ENDPOINT="$minio_endpoint" \
    -e MC_ACCESS_KEY="$minio_access_key" \
    -e MC_SECRET_KEY="$minio_secret_key" \
    -e MC_BUCKET="$minio_bucket" \
    -e MC_PREFIX="$object_prefix" \
    -e MC_INDEX="$index" \
    "$mc_image" -ec '
      mc alias set local "$MC_ENDPOINT" "$MC_ACCESS_KEY" "$MC_SECRET_KEY" >/dev/null
      mc cp /work/object.bin "local/$MC_BUCKET/$MC_PREFIX/object-$MC_INDEX.bin" >/dev/null
    ' >"$temporary/upload-$index.log" 2>&1 &
  upload_pids+=("$!")
done
upload_failures=0
for ((index = 1; index <= object_copies; index++)); do
  if ! wait "${upload_pids[$((index - 1))]}"; then
    upload_failures=$((upload_failures + 1))
    tail -40 "$temporary/upload-$index.log" >&2
  fi
done
upload_completed_ns=$(now_ns)
((upload_failures == 0)) || {
  echo "验收失败: $upload_failures/$object_copies 个对象上传失败" >&2
  exit 1
}

download_started_ns=$(now_ns)
download_pids=()
for ((index = 1; index <= object_copies; index++)); do
  docker run --rm --network "$docker_network" \
    --entrypoint /bin/sh \
    -v "$temporary/downloads:/downloads" \
    -e MC_ENDPOINT="$minio_endpoint" \
    -e MC_ACCESS_KEY="$minio_access_key" \
    -e MC_SECRET_KEY="$minio_secret_key" \
    -e MC_BUCKET="$minio_bucket" \
    -e MC_PREFIX="$object_prefix" \
    -e MC_INDEX="$index" \
    "$mc_image" -ec '
      mc alias set local "$MC_ENDPOINT" "$MC_ACCESS_KEY" "$MC_SECRET_KEY" >/dev/null
      mc cp "local/$MC_BUCKET/$MC_PREFIX/object-$MC_INDEX.bin" "/downloads/object-$MC_INDEX.bin" >/dev/null
    ' >"$temporary/download-$index.log" 2>&1 &
  download_pids+=("$!")
done
download_failures=0
for ((index = 1; index <= object_copies; index++)); do
  if ! wait "${download_pids[$((index - 1))]}"; then
    download_failures=$((download_failures + 1))
    tail -40 "$temporary/download-$index.log" >&2
    continue
  fi
  downloaded_sha256=$(sha256_file "$temporary/downloads/object-$index.bin")
  if [[ $downloaded_sha256 != "$object_sha256" ]]; then
    echo "验收失败: 下载对象 $index 的 SHA-256 不一致" >&2
    download_failures=$((download_failures + 1))
  fi
done
download_completed_ns=$(now_ns)
((download_failures == 0)) || {
  echo "验收失败: $download_failures/$object_copies 个对象下载或摘要校验失败" >&2
  exit 1
}

object_total_mib=$((object_mib * object_copies))
upload_duration=$(duration_seconds "$upload_started_ns" "$upload_completed_ns")
download_duration=$(duration_seconds "$download_started_ns" "$download_completed_ns")
upload_mibps=$(rate_per_second "$object_total_mib" "$upload_duration")
download_mibps=$(rate_per_second "$object_total_mib" "$download_duration")
python3 - "$upload_mibps" "$download_mibps" "$object_min_mibps" <<'PY'
import sys
upload, download, minimum = map(float, sys.argv[1:])
if upload < minimum or download < minimum:
    raise SystemExit(
        f"验收失败: 对象吞吐 upload={upload:.3f}MiB/s download={download:.3f}MiB/s，门禁={minimum:.3f}MiB/s"
    )
PY

cleanup_object_prefix
object_prefix_active=false

echo "开始: API 持续并发稳定性，时长=${soak_seconds}s 并发=$soak_concurrency"
APPFORGE_SOAK_URL="$api_url/healthz" \
APPFORGE_SOAK_SECONDS="$soak_seconds" \
APPFORGE_SOAK_CONCURRENCY="$soak_concurrency" \
APPFORGE_SOAK_P99_LIMIT="$soak_p99_limit" \
APPFORGE_SOAK_OUTPUT="$temporary/soak.json" \
python3 <<'PY'
import concurrent.futures
import json
import math
import os
import statistics
import threading
import time
import urllib.request

url = os.environ["APPFORGE_SOAK_URL"]
duration = int(os.environ["APPFORGE_SOAK_SECONDS"])
concurrency = int(os.environ["APPFORGE_SOAK_CONCURRENCY"])
p99_limit = float(os.environ["APPFORGE_SOAK_P99_LIMIT"])
deadline = time.monotonic() + duration
latencies = []
errors = []
lock = threading.Lock()

def worker():
    local_latencies = []
    local_errors = []
    while time.monotonic() < deadline:
        started = time.monotonic()
        try:
            with urllib.request.urlopen(url, timeout=5) as response:
                response.read()
                if response.status != 200:
                    local_errors.append(f"http-{response.status}")
                else:
                    local_latencies.append(time.monotonic() - started)
        except Exception as exc:
            local_errors.append(type(exc).__name__)
    with lock:
        latencies.extend(local_latencies)
        errors.extend(local_errors)

with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
    list(executor.map(lambda _: worker(), range(concurrency)))

def percentile(values, percentage):
    if not values:
        return 0.0
    ordered = sorted(values)
    index = max(0, math.ceil(len(ordered) * percentage) - 1)
    return ordered[index]

result = {
    "durationSeconds": duration,
    "concurrency": concurrency,
    "requests": len(latencies) + len(errors),
    "successes": len(latencies),
    "errors": len(errors),
    "averageSeconds": statistics.fmean(latencies) if latencies else 0.0,
    "p50Seconds": percentile(latencies, 0.50),
    "p95Seconds": percentile(latencies, 0.95),
    "p99Seconds": percentile(latencies, 0.99),
    "maxSeconds": max(latencies) if latencies else 0.0,
}
with open(os.environ["APPFORGE_SOAK_OUTPUT"], "w", encoding="utf-8") as output:
    json.dump(result, output, ensure_ascii=False, indent=2)
    output.write("\n")
if result["errors"] != 0:
    raise SystemExit(f"验收失败: API 持续探针错误数={result['errors']}")
if result["requests"] < concurrency:
    raise SystemExit(f"验收失败: API 持续探针请求数不足={result['requests']}")
if result["p99Seconds"] > p99_limit:
    raise SystemExit(
        f"验收失败: API 持续探针 P99={result['p99Seconds']:.6f}s 超过门禁 {p99_limit:.6f}s"
    )
PY

soak_requests=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["requests"])' "$temporary/soak.json")
soak_errors=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["errors"])' "$temporary/soak.json")
soak_p99=$(python3 -c 'import json,sys; print("{:.6f}".format(json.load(open(sys.argv[1]))["p99Seconds"]))' "$temporary/soak.json")

mkdir -p "$(dirname "$report_file")"
umask 077
APPFORGE_CAPACITY_ACCEPTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
APPFORGE_CAPACITY_AGENT_IMAGE="$agent_image" \
APPFORGE_CAPACITY_BUILD_CONCURRENCY="$build_concurrency" \
APPFORGE_CAPACITY_BUILD_DURATION="$build_duration" \
APPFORGE_CAPACITY_BUILDS_PER_MINUTE="$builds_per_minute" \
APPFORGE_CAPACITY_OBJECT_MIB="$object_mib" \
APPFORGE_CAPACITY_OBJECT_COPIES="$object_copies" \
APPFORGE_CAPACITY_OBJECT_TOTAL_MIB="$object_total_mib" \
APPFORGE_CAPACITY_UPLOAD_DURATION="$upload_duration" \
APPFORGE_CAPACITY_UPLOAD_MIBPS="$upload_mibps" \
APPFORGE_CAPACITY_DOWNLOAD_DURATION="$download_duration" \
APPFORGE_CAPACITY_DOWNLOAD_MIBPS="$download_mibps" \
APPFORGE_CAPACITY_OBJECT_MIN_MIBPS="$object_min_mibps" \
APPFORGE_CAPACITY_SOAK_FILE="$temporary/soak.json" \
APPFORGE_CAPACITY_SOAK_P99_LIMIT="$soak_p99_limit" \
python3 <<'PY' >"$report_file"
import json
import os
import sys

integer = lambda key: int(os.environ[key])
number = lambda key: float(os.environ[key])
with open(os.environ["APPFORGE_CAPACITY_SOAK_FILE"], encoding="utf-8") as source:
    soak = json.load(source)
json.dump({
    "schemaVersion": 1,
    "acceptedAt": os.environ["APPFORGE_CAPACITY_ACCEPTED_AT"],
    "scope": "synthetic-local",
    "acceptanceScript": "deploy/acceptance/v7-artifact-capacity.sh",
    "notCustomerCapacity": True,
    "apkBuild": {
        "image": os.environ["APPFORGE_CAPACITY_AGENT_IMAGE"],
        "concurrency": integer("APPFORGE_CAPACITY_BUILD_CONCURRENCY"),
        "completed": integer("APPFORGE_CAPACITY_BUILD_CONCURRENCY"),
        "failed": 0,
        "durationSeconds": number("APPFORGE_CAPACITY_BUILD_DURATION"),
        "buildsPerMinute": number("APPFORGE_CAPACITY_BUILDS_PER_MINUTE"),
        "validation": ["apk-signature", "package-name", "result-sha256"],
    },
    "localObjectStorage": {
        "provider": "temporary-prefix-on-development-minio",
        "objectMiB": integer("APPFORGE_CAPACITY_OBJECT_MIB"),
        "copies": integer("APPFORGE_CAPACITY_OBJECT_COPIES"),
        "totalMiBPerDirection": integer("APPFORGE_CAPACITY_OBJECT_TOTAL_MIB"),
        "uploadDurationSeconds": number("APPFORGE_CAPACITY_UPLOAD_DURATION"),
        "uploadMiBPerSecond": number("APPFORGE_CAPACITY_UPLOAD_MIBPS"),
        "downloadDurationSeconds": number("APPFORGE_CAPACITY_DOWNLOAD_DURATION"),
        "downloadMiBPerSecond": number("APPFORGE_CAPACITY_DOWNLOAD_MIBPS"),
        "minimumMiBPerSecond": number("APPFORGE_CAPACITY_OBJECT_MIN_MIBPS"),
        "sha256Verified": True,
        "temporaryPrefixRemoved": True,
    },
    "apiStability": {**soak, "p99LimitSeconds": number("APPFORGE_CAPACITY_SOAK_P99_LIMIT")},
    "limitations": [
        "synthetic-apk-and-test-keystore-only",
        "single-developer-workstation",
        "local-docker-network-and-development-minio",
        "short-stability-run-not-production-soak",
        "not-an-android-authorized-device-test",
        "not-a-customer-capacity-or-peak-commitment",
    ],
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
PY
chmod 0600 "$report_file"

echo "通过: 合成APK构建=${build_concurrency}/${build_concurrency} 耗时=${build_duration}s 速率=${builds_per_minute}次/分钟"
echo "通过: 本地MinIO上传=${upload_mibps}MiB/s 下载=${download_mibps}MiB/s SHA-256一致"
echo "通过: API持续探针=${soak_requests}次/${soak_seconds}s 并发=${soak_concurrency} 错误=${soak_errors} P99=${soak_p99}s"
echo "证据: $report_file"
echo "边界: 该结果仅为合成本地容量证据，不是客户容量、峰值、长稳或生产承诺"
