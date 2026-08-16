#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
duration_seconds=${APPFORGE_ARTIFACT_SOAK_DURATION_SECONDS:-3600}
build_concurrency=${APPFORGE_ARTIFACT_SOAK_BUILD_CONCURRENCY:-1}
build_interval_seconds=${APPFORGE_ARTIFACT_SOAK_BUILD_INTERVAL_SECONDS:-10}
minimum_builds=${APPFORGE_ARTIFACT_SOAK_MIN_BUILDS:-0}
apk_payload_mib=${APPFORGE_ARTIFACT_SOAK_APK_PAYLOAD_MIB:-0}
object_concurrency=${APPFORGE_ARTIFACT_SOAK_OBJECT_CONCURRENCY:-1}
object_interval_seconds=${APPFORGE_ARTIFACT_SOAK_OBJECT_INTERVAL_SECONDS:-1}
object_mib=${APPFORGE_ARTIFACT_SOAK_OBJECT_MIB:-1}
minimum_object_round_trips=${APPFORGE_ARTIFACT_SOAK_MIN_OBJECT_ROUND_TRIPS:-0}
docker_network=${APPFORGE_ARTIFACT_SOAK_DOCKER_NETWORK:-appforge-dev}
agent_image=${APPFORGE_ARTIFACT_SOAK_AGENT_IMAGE:-appforge-local-agent:soak}
mc_image=${APPFORGE_ARTIFACT_SOAK_MC_IMAGE:-minio/mc:RELEASE.2025-04-16T18-13-26Z}
minio_endpoint=${APPFORGE_ARTIFACT_SOAK_MINIO_ENDPOINT:-http://minio:9000}
minio_bucket=${APPFORGE_ARTIFACT_SOAK_MINIO_BUCKET:-appforge}
minio_access_key_file=${APPFORGE_ARTIFACT_SOAK_MINIO_ACCESS_KEY_FILE:-}
minio_secret_key_file=${APPFORGE_ARTIFACT_SOAK_MINIO_SECRET_KEY_FILE:-}
report_file=${APPFORGE_ARTIFACT_SOAK_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-artifact-soak-20260817.json}
environment_kind=${APPFORGE_ARTIFACT_SOAK_ENVIRONMENT_KIND:-fixture}

require_integer() {
  local name=$1 value=$2 minimum=$3 maximum=$4
  [[ $value =~ ^[0-9]+$ ]] && ((value >= minimum && value <= maximum)) || {
    echo "验收失败: $name 必须在 $minimum 到 $maximum 之间" >&2
    exit 1
  }
}

require_private_file() {
  local name=$1 path=$2 mode
  [[ -n $path && $path == /* && -f $path && ! -L $path ]] || {
    echo "验收失败: $name 必须是绝对路径普通文件" >&2
    exit 1
  }
  mode=$(stat -c %a "$path" 2>/dev/null || stat -f %Lp "$path")
  (( (8#$mode & 8#077) == 0 )) || {
    echo "验收失败: $name 不能向 group/others 开放" >&2
    exit 1
  }
  [[ -s $path ]] || { echo "验收失败: $name 不能为空" >&2; exit 1; }
}

require_integer APPFORGE_ARTIFACT_SOAK_DURATION_SECONDS "$duration_seconds" 10 86400
require_integer APPFORGE_ARTIFACT_SOAK_BUILD_CONCURRENCY "$build_concurrency" 1 8
require_integer APPFORGE_ARTIFACT_SOAK_BUILD_INTERVAL_SECONDS "$build_interval_seconds" 0 3600
require_integer APPFORGE_ARTIFACT_SOAK_MIN_BUILDS "$minimum_builds" 0 1000000
require_integer APPFORGE_ARTIFACT_SOAK_APK_PAYLOAD_MIB "$apk_payload_mib" 0 256
require_integer APPFORGE_ARTIFACT_SOAK_OBJECT_CONCURRENCY "$object_concurrency" 1 16
require_integer APPFORGE_ARTIFACT_SOAK_OBJECT_INTERVAL_SECONDS "$object_interval_seconds" 0 3600
require_integer APPFORGE_ARTIFACT_SOAK_OBJECT_MIB "$object_mib" 1 256
require_integer APPFORGE_ARTIFACT_SOAK_MIN_OBJECT_ROUND_TRIPS "$minimum_object_round_trips" 0 10000000
[[ $environment_kind == fixture || $environment_kind == customer-test ]] || {
  echo "验收失败: APPFORGE_ARTIFACT_SOAK_ENVIRONMENT_KIND 必须是 fixture 或 customer-test" >&2
  exit 1
}
[[ $report_file == /* ]] || { echo "验收失败: 证据路径必须是绝对路径" >&2; exit 1; }
[[ $docker_network =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$ ]] || {
  echo "验收失败: Docker 网络名称不安全" >&2
  exit 1
}
[[ $agent_image =~ ^[A-Za-z0-9][A-Za-z0-9_./:@+-]{0,255}$ ]] || {
  echo "验收失败: Local Agent 镜像引用不安全" >&2
  exit 1
}
[[ $mc_image =~ ^[A-Za-z0-9][A-Za-z0-9_./:@+-]{0,255}$ ]] || {
  echo "验收失败: MinIO Client 镜像引用不安全" >&2
  exit 1
}
[[ $minio_endpoint =~ ^https?://([A-Za-z0-9.-]+|\[[0-9A-Fa-f:]+\])(:[0-9]{1,5})?$ ]] || {
  echo "验收失败: MinIO Endpoint 必须是无凭据、路径、查询和片段的 HTTP(S) Origin" >&2
  exit 1
}
[[ $minio_bucket =~ ^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$ ]] || {
  echo "验收失败: MinIO Bucket 名称不安全" >&2
  exit 1
}
require_private_file APPFORGE_ARTIFACT_SOAK_MINIO_ACCESS_KEY_FILE "$minio_access_key_file"
require_private_file APPFORGE_ARTIFACT_SOAK_MINIO_SECRET_KEY_FILE "$minio_secret_key_file"

for command_name in awk date dd docker python3 sha256sum; do
  command -v "$command_name" >/dev/null || {
    echo "验收失败: 缺少命令 $command_name" >&2
    exit 1
  }
done
docker network inspect "$docker_network" >/dev/null
docker image inspect "$agent_image" >/dev/null
docker image inspect "$mc_image" >/dev/null

temporary=$(mktemp -d /tmp/appforge-v7-artifact-soak.XXXXXX)
suffix="$(date +%s)-$$"
object_prefix="acceptance/v7-artifact-soak-$suffix"
worker_pids=()
object_prefix_active=true

cleanup_object_prefix() {
  [[ $object_prefix_active == true ]] || return 0
  docker run --rm --network "$docker_network" \
    --entrypoint /bin/sh \
    -v "$minio_access_key_file:/run/secrets/minio-access-key:ro" \
    -v "$minio_secret_key_file:/run/secrets/minio-secret-key:ro" \
    -e MC_ENDPOINT="$minio_endpoint" \
    -e MC_BUCKET="$minio_bucket" \
    -e MC_PREFIX="$object_prefix" \
    "$mc_image" -ec '
      access_key=$(cat /run/secrets/minio-access-key)
      secret_key=$(cat /run/secrets/minio-secret-key)
      mc alias set local "$MC_ENDPOINT" "$access_key" "$secret_key" >/dev/null
      unset access_key secret_key
      mc rm --recursive --force "local/$MC_BUCKET/$MC_PREFIX/" >/dev/null 2>&1 || true
      test -z "$(mc find "local/$MC_BUCKET/$MC_PREFIX/" --maxdepth 1 2>/dev/null | head -1)"
    ' >/dev/null
}

cleanup() {
  set +e
  for pid in "${worker_pids[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  for pid in "${worker_pids[@]:-}"; do
    wait "$pid" >/dev/null 2>&1 || true
  done
  cleanup_object_prefix >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT INT TERM

dd if=/dev/urandom of="$temporary/object.bin" bs=1048576 count="$object_mib" 2>/dev/null
object_sha256=$(sha256sum "$temporary/object.bin" | awk '{print $1}')
object_size_bytes=$((object_mib * 1048576))
chmod 0444 "$temporary/object.bin"
mkdir -p "$temporary/downloads"
chmod 0777 "$temporary/downloads"

if ((minimum_builds == 0)); then
  minimum_builds=$((duration_seconds * build_concurrency / (build_interval_seconds + 15)))
  ((minimum_builds >= 1)) || minimum_builds=1
fi
if ((minimum_object_round_trips == 0)); then
  minimum_object_round_trips=$((duration_seconds * object_concurrency / (object_interval_seconds + 5)))
  ((minimum_object_round_trips >= 1)) || minimum_object_round_trips=1
fi

now_ns() {
  python3 -c 'import time; print(time.time_ns())'
}

build_worker() {
  local worker_index=$1 result_file="$temporary/build-$1.results" log_file="$temporary/build-$1.log"
  local worker_started_ns worker_deadline_ns cycle_started_ns cycle_completed_ns sleep_seconds
  worker_started_ns=$(now_ns)
  worker_deadline_ns=$((worker_started_ns + duration_seconds * 1000000000))
  while (( $(now_ns) < worker_deadline_ns )); do
    cycle_started_ns=$(now_ns)
    if APPFORGE_LOCAL_AGENT_IMAGE="$agent_image" \
      APPFORGE_LOCAL_AGENT_SYNTHETIC_PAYLOAD_MIB="$apk_payload_mib" \
      "$repo_root/deploy/acceptance/local-agent-executor.sh" >>"$log_file" 2>&1; then
      cycle_completed_ns=$(now_ns)
      printf 'OK %s\n' "$((cycle_completed_ns - cycle_started_ns))" >>"$result_file"
    else
      cycle_completed_ns=$(now_ns)
      printf 'FAIL %s\n' "$((cycle_completed_ns - cycle_started_ns))" >>"$result_file"
    fi
    if ((build_interval_seconds > 0 && $(now_ns) < worker_deadline_ns)); then
      sleep_seconds=$build_interval_seconds
      sleep "$sleep_seconds"
    fi
  done
}

object_worker() {
  local worker_index=$1 result_file="$temporary/object-$1.results"
  docker run --rm --network "$docker_network" \
    --entrypoint /bin/sh \
    -v "$temporary/object.bin:/work/object.bin:ro" \
    -v "$temporary/downloads:/downloads" \
    -v "$minio_access_key_file:/run/secrets/minio-access-key:ro" \
    -v "$minio_secret_key_file:/run/secrets/minio-secret-key:ro" \
    -e MC_ENDPOINT="$minio_endpoint" \
    -e MC_BUCKET="$minio_bucket" \
    -e MC_PREFIX="$object_prefix" \
    -e WORKER_INDEX="$worker_index" \
    -e DURATION_SECONDS="$duration_seconds" \
    -e INTERVAL_SECONDS="$object_interval_seconds" \
    -e EXPECTED_SHA256="$object_sha256" \
    -e OBJECT_SIZE_BYTES="$object_size_bytes" \
    "$mc_image" -ec '
      access_key=$(cat /run/secrets/minio-access-key)
      secret_key=$(cat /run/secrets/minio-secret-key)
      mc alias set local "$MC_ENDPOINT" "$access_key" "$secret_key" >/dev/null
      unset access_key secret_key
      started=$(date +%s)
      deadline=$((started + DURATION_SECONDS))
      sequence=0
      while [ "$(date +%s)" -lt "$deadline" ]; do
        sequence=$((sequence + 1))
        key="object-$WORKER_INDEX-$sequence.bin"
        remote="local/$MC_BUCKET/$MC_PREFIX/$key"
        downloaded="/downloads/$key"
        failed=0
        mc cp /work/object.bin "$remote" >/dev/null || failed=1
        if [ "$failed" -eq 0 ]; then
          mc stat "$remote" >/dev/null || failed=1
        fi
        if [ "$failed" -eq 0 ]; then
          mc cp "$remote" "$downloaded" >/dev/null || failed=1
        fi
        if [ "$failed" -eq 0 ]; then
          actual_sha256=$(sha256sum "$downloaded" | cut -d " " -f 1)
          [ "$actual_sha256" = "$EXPECTED_SHA256" ] || failed=1
        fi
        mc rm --force "$remote" >/dev/null 2>&1 || failed=1
        if mc stat "$remote" >/dev/null 2>&1; then
          failed=1
        fi
        rm -f "$downloaded"
        if [ "$failed" -eq 0 ]; then
          echo "OK $OBJECT_SIZE_BYTES"
        else
          echo "FAIL $OBJECT_SIZE_BYTES"
        fi
        if [ "$INTERVAL_SECONDS" -gt 0 ] && [ "$(date +%s)" -lt "$deadline" ]; then
          sleep "$INTERVAL_SECONDS"
        fi
      done
    ' >"$result_file" 2>&1
}

started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
started_ns=$(now_ns)
echo "开始: 合成 APK 与 MinIO 对象混合长稳，时长=${duration_seconds}s 构建并发=$build_concurrency 对象并发=$object_concurrency"

for ((index = 1; index <= build_concurrency; index++)); do
  build_worker "$index" &
  worker_pids+=("$!")
done
for ((index = 1; index <= object_concurrency; index++)); do
  object_worker "$index" &
  worker_pids+=("$!")
done

worker_process_failures=0
for pid in "${worker_pids[@]}"; do
  if ! wait "$pid"; then
    worker_process_failures=$((worker_process_failures + 1))
  fi
done
worker_pids=()
completed_ns=$(now_ns)

builds_completed=$(awk '/^OK / {count++} END {print count+0}' "$temporary"/build-*.results)
build_failures=$(awk '/^FAIL / {count++} END {print count+0}' "$temporary"/build-*.results)
build_total_ns=$(awk '/^OK / {sum+=$2} END {printf "%.0f", sum+0}' "$temporary"/build-*.results)
object_round_trips=$(awk '/^OK / {count++} END {print count+0}' "$temporary"/object-*.results)
object_failures=$(awk '/^FAIL / {count++} END {print count+0}' "$temporary"/object-*.results)
actual_duration_seconds=$(python3 - "$started_ns" "$completed_ns" <<'PY'
import sys
started, completed = map(int, sys.argv[1:])
print(f"{(completed - started) / 1_000_000_000:.6f}")
PY
)

cleanup_object_prefix
object_prefix_active=false

passed=true
((worker_process_failures == 0)) || passed=false
((build_failures == 0)) || passed=false
((object_failures == 0)) || passed=false
((builds_completed >= minimum_builds)) || passed=false
((object_round_trips >= minimum_object_round_trips)) || passed=false

mkdir -p "$(dirname "$report_file")"
umask 077
APPFORGE_ARTIFACT_SOAK_ACCEPTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
APPFORGE_ARTIFACT_SOAK_STARTED_AT="$started_at" \
APPFORGE_ARTIFACT_SOAK_RESULT="$passed" \
APPFORGE_ARTIFACT_SOAK_ENVIRONMENT_KIND="$environment_kind" \
APPFORGE_ARTIFACT_SOAK_DURATION_SECONDS="$duration_seconds" \
APPFORGE_ARTIFACT_SOAK_ACTUAL_DURATION_SECONDS="$actual_duration_seconds" \
APPFORGE_ARTIFACT_SOAK_AGENT_IMAGE="$agent_image" \
APPFORGE_ARTIFACT_SOAK_BUILD_CONCURRENCY="$build_concurrency" \
APPFORGE_ARTIFACT_SOAK_APK_PAYLOAD_MIB="$apk_payload_mib" \
APPFORGE_ARTIFACT_SOAK_BUILDS_COMPLETED="$builds_completed" \
APPFORGE_ARTIFACT_SOAK_BUILD_FAILURES="$build_failures" \
APPFORGE_ARTIFACT_SOAK_MIN_BUILDS="$minimum_builds" \
APPFORGE_ARTIFACT_SOAK_BUILD_TOTAL_NS="$build_total_ns" \
APPFORGE_ARTIFACT_SOAK_OBJECT_CONCURRENCY="$object_concurrency" \
APPFORGE_ARTIFACT_SOAK_OBJECT_MIB="$object_mib" \
APPFORGE_ARTIFACT_SOAK_OBJECT_ROUND_TRIPS="$object_round_trips" \
APPFORGE_ARTIFACT_SOAK_OBJECT_FAILURES="$object_failures" \
APPFORGE_ARTIFACT_SOAK_MIN_OBJECT_ROUND_TRIPS="$minimum_object_round_trips" \
APPFORGE_ARTIFACT_SOAK_OBJECT_SIZE_BYTES="$object_size_bytes" \
APPFORGE_ARTIFACT_SOAK_ENDPOINT_SHA256="$(printf '%s' "${minio_endpoint,,}" | sha256sum | awk '{print $1}')" \
python3 <<'PY' >"$report_file"
import json
import os
import sys

integer = lambda key: int(os.environ[key])
number = lambda key: float(os.environ[key])
actual_duration = number("APPFORGE_ARTIFACT_SOAK_ACTUAL_DURATION_SECONDS")
builds = integer("APPFORGE_ARTIFACT_SOAK_BUILDS_COMPLETED")
build_total_seconds = integer("APPFORGE_ARTIFACT_SOAK_BUILD_TOTAL_NS") / 1_000_000_000
round_trips = integer("APPFORGE_ARTIFACT_SOAK_OBJECT_ROUND_TRIPS")
object_size = integer("APPFORGE_ARTIFACT_SOAK_OBJECT_SIZE_BYTES")
result = os.environ["APPFORGE_ARTIFACT_SOAK_RESULT"] == "true"
json.dump({
    "schemaVersion": 1,
    "evidenceType": "v7-artifact-mixed-soak",
    "acceptedAt": os.environ["APPFORGE_ARTIFACT_SOAK_ACCEPTED_AT"],
    "result": "passed" if result else "failed",
    "runClass": "day" if actual_duration >= 86400 else "hour" if actual_duration >= 3600 else "smoke",
    "qualifiesHourLevel": actual_duration >= 3600,
    "qualifiesDayLevel": actual_duration >= 86400,
    "environmentKind": os.environ["APPFORGE_ARTIFACT_SOAK_ENVIRONMENT_KIND"],
    "startedAt": os.environ["APPFORGE_ARTIFACT_SOAK_STARTED_AT"],
    "configuredDurationSeconds": integer("APPFORGE_ARTIFACT_SOAK_DURATION_SECONDS"),
    "actualDurationSeconds": actual_duration,
    "apkBuild": {
        "image": os.environ["APPFORGE_ARTIFACT_SOAK_AGENT_IMAGE"],
        "concurrency": integer("APPFORGE_ARTIFACT_SOAK_BUILD_CONCURRENCY"),
        "syntheticPayloadMiB": integer("APPFORGE_ARTIFACT_SOAK_APK_PAYLOAD_MIB"),
        "sourcePayloadBytesProcessed": builds * integer("APPFORGE_ARTIFACT_SOAK_APK_PAYLOAD_MIB") * 1024 * 1024,
        "completed": builds,
        "failed": integer("APPFORGE_ARTIFACT_SOAK_BUILD_FAILURES"),
        "minimumCompleted": integer("APPFORGE_ARTIFACT_SOAK_MIN_BUILDS"),
        "averageBuildSeconds": build_total_seconds / builds if builds else 0,
        "buildsPerMinute": builds * 60 / actual_duration if actual_duration else 0,
        "validation": ["apk-signature", "package-name", "result-sha256"] + (
            ["synthetic-payload-sha256-preserved"]
            if integer("APPFORGE_ARTIFACT_SOAK_APK_PAYLOAD_MIB") > 0 else []
        ),
    },
    "objectStorage": {
        "provider": "minio",
        "endpointSha256": os.environ["APPFORGE_ARTIFACT_SOAK_ENDPOINT_SHA256"],
        "concurrency": integer("APPFORGE_ARTIFACT_SOAK_OBJECT_CONCURRENCY"),
        "objectMiB": integer("APPFORGE_ARTIFACT_SOAK_OBJECT_MIB"),
        "roundTrips": round_trips,
        "failed": integer("APPFORGE_ARTIFACT_SOAK_OBJECT_FAILURES"),
        "minimumRoundTrips": integer("APPFORGE_ARTIFACT_SOAK_MIN_OBJECT_ROUND_TRIPS"),
        "bytesUploaded": round_trips * object_size,
        "bytesDownloaded": round_trips * object_size,
        "sha256Verified": True,
        "deleteNotFoundVerified": True,
        "temporaryPrefixRemoved": True,
    },
    "verified": [
        "synthetic-apk-build-and-sign",
        "object-put-stat-get-sha256-delete-not-found",
        "zero-build-errors",
        "zero-object-errors",
        "minimum-operation-counts",
        "credential-files-not-reported",
        "temporary-prefix-removed",
    ],
    "limitations": [
        "synthetic-apk-and-test-keystore-only",
        "synthetic-apk-payload-is-random-test-data-not-a-customer-application",
        "does-not-use-customer-or-production-data",
        "does-not-represent-customer-peak-or-production-capacity",
        "does-not-replace-day-level-soak-unless-explicitly-qualified",
        "not-an-android-authorized-device-test",
    ],
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
PY
chmod 0600 "$report_file"

if [[ $passed != true ]]; then
  echo "验收失败: 进程失败=$worker_process_failures 构建=${builds_completed}/${minimum_builds} 构建错误=$build_failures 对象=${object_round_trips}/${minimum_object_round_trips} 对象错误=$object_failures" >&2
  for log_file in "$temporary"/build-*.log "$temporary"/object-*.results; do
    [[ -f $log_file ]] && tail -20 "$log_file" >&2
  done
  echo "失败证据: $report_file" >&2
  exit 1
fi

echo "通过: 合成 APK 构建=${builds_completed}/${minimum_builds} 错误=0；对象往返=${object_round_trips}/${minimum_object_round_trips} 错误=0"
echo "证据: $report_file"
echo "边界: 仅为合成混合长稳，不是客户峰值、生产容量、授权 Android 设备或天级承诺"
