#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
api_url=${APPFORGE_SOAK_API_URL:-http://127.0.0.1:8888}
admin_username=${APPFORGE_SOAK_ADMIN_USERNAME:-appforge}
password_file=${APPFORGE_SOAK_ADMIN_PASSWORD_FILE:-}
ca_file=${APPFORGE_SOAK_CA_FILE:-}
duration_seconds=${APPFORGE_SOAK_DURATION_SECONDS:-3600}
health_concurrency=${APPFORGE_SOAK_HEALTH_CONCURRENCY:-2}
ready_concurrency=${APPFORGE_SOAK_READY_CONCURRENCY:-2}
deployment_concurrency=${APPFORGE_SOAK_DEPLOYMENT_CONCURRENCY:-2}
health_interval_ms=${APPFORGE_SOAK_HEALTH_INTERVAL_MS:-500}
ready_interval_ms=${APPFORGE_SOAK_READY_INTERVAL_MS:-500}
deployment_interval_ms=${APPFORGE_SOAK_DEPLOYMENT_INTERVAL_MS:-1000}
health_p99_limit=${APPFORGE_SOAK_HEALTH_P99_SECONDS:-0.5}
ready_p99_limit=${APPFORGE_SOAK_READY_P99_SECONDS:-0.5}
deployment_p99_limit=${APPFORGE_SOAK_DEPLOYMENT_P99_SECONDS:-2.5}
max_errors=${APPFORGE_SOAK_MAX_ERRORS:-0}
max_consecutive_errors=${APPFORGE_SOAK_MAX_CONSECUTIVE_ERRORS:-0}
report_file=${APPFORGE_SOAK_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-api-soak-20260817.json}
environment_kind=${APPFORGE_SOAK_ENVIRONMENT_KIND:-fixture}

require_integer() {
  local name=$1 value=$2 minimum=$3 maximum=$4
  [[ $value =~ ^[0-9]+$ ]] && (( value >= minimum && value <= maximum )) || {
    echo "验收失败: $name 必须在 $minimum 到 $maximum 之间" >&2
    exit 1
  }
}

require_number() {
  local name=$1 value=$2
  [[ $value =~ ^[0-9]+([.][0-9]+)?$ ]] || {
    echo "验收失败: $name 必须是非负数" >&2
    exit 1
  }
}

[[ $api_url =~ ^https?://([A-Za-z0-9.-]+|\[[0-9A-Fa-f:]+\])(:[0-9]{1,5})?$ ]] || {
  echo "验收失败: APPFORGE_SOAK_API_URL 必须是无凭据、路径、查询和片段的 HTTP(S) Origin" >&2
  exit 1
}
[[ $report_file == /* ]] || { echo "验收失败: 证据路径必须是绝对路径" >&2; exit 1; }
[[ $environment_kind == fixture || $environment_kind == customer-test ]] || {
  echo "验收失败: APPFORGE_SOAK_ENVIRONMENT_KIND 必须是 fixture 或 customer-test" >&2
  exit 1
}
[[ -n $password_file && $password_file == /* && -f $password_file && ! -L $password_file ]] || {
  echo "验收失败: 管理员密码必须来自绝对路径普通文件" >&2
  exit 1
}
password_mode=$(stat -c %a "$password_file" 2>/dev/null || stat -f %Lp "$password_file")
(( (8#$password_mode & 8#077) == 0 )) || { echo "验收失败: 管理员密码文件不能向 group/others 开放" >&2; exit 1; }
if [[ -n $ca_file ]]; then
  [[ $ca_file == /* && -f $ca_file && ! -L $ca_file ]] || { echo "验收失败: CA 必须是绝对路径普通文件" >&2; exit 1; }
fi
require_integer APPFORGE_SOAK_DURATION_SECONDS "$duration_seconds" 10 86400
require_integer APPFORGE_SOAK_HEALTH_CONCURRENCY "$health_concurrency" 1 64
require_integer APPFORGE_SOAK_READY_CONCURRENCY "$ready_concurrency" 1 64
require_integer APPFORGE_SOAK_DEPLOYMENT_CONCURRENCY "$deployment_concurrency" 1 64
require_integer APPFORGE_SOAK_HEALTH_INTERVAL_MS "$health_interval_ms" 50 60000
require_integer APPFORGE_SOAK_READY_INTERVAL_MS "$ready_interval_ms" 50 60000
require_integer APPFORGE_SOAK_DEPLOYMENT_INTERVAL_MS "$deployment_interval_ms" 50 60000
require_integer APPFORGE_SOAK_MAX_ERRORS "$max_errors" 0 1000000
require_integer APPFORGE_SOAK_MAX_CONSECUTIVE_ERRORS "$max_consecutive_errors" 0 1000000
for pair in \
  "APPFORGE_SOAK_HEALTH_P99_SECONDS:$health_p99_limit" \
  "APPFORGE_SOAK_READY_P99_SECONDS:$ready_p99_limit" \
  "APPFORGE_SOAK_DEPLOYMENT_P99_SECONDS:$deployment_p99_limit"; do
  require_number "${pair%%:*}" "${pair#*:}"
done

temporary=$(mktemp -d /tmp/appforge-v7-api-soak.XXXXXX)
cleanup() { rm -rf "$temporary"; }
trap cleanup EXIT
mkdir -p "$(dirname "$report_file")"
umask 077

APPFORGE_SOAK_API_URL="$api_url" \
APPFORGE_SOAK_ADMIN_USERNAME="$admin_username" \
APPFORGE_SOAK_ADMIN_PASSWORD_FILE="$password_file" \
APPFORGE_SOAK_CA_FILE="$ca_file" \
APPFORGE_SOAK_DURATION_SECONDS="$duration_seconds" \
APPFORGE_SOAK_HEALTH_CONCURRENCY="$health_concurrency" \
APPFORGE_SOAK_READY_CONCURRENCY="$ready_concurrency" \
APPFORGE_SOAK_DEPLOYMENT_CONCURRENCY="$deployment_concurrency" \
APPFORGE_SOAK_HEALTH_INTERVAL_MS="$health_interval_ms" \
APPFORGE_SOAK_READY_INTERVAL_MS="$ready_interval_ms" \
APPFORGE_SOAK_DEPLOYMENT_INTERVAL_MS="$deployment_interval_ms" \
APPFORGE_SOAK_HEALTH_P99_SECONDS="$health_p99_limit" \
APPFORGE_SOAK_READY_P99_SECONDS="$ready_p99_limit" \
APPFORGE_SOAK_DEPLOYMENT_P99_SECONDS="$deployment_p99_limit" \
APPFORGE_SOAK_MAX_ERRORS="$max_errors" \
APPFORGE_SOAK_MAX_CONSECUTIVE_ERRORS="$max_consecutive_errors" \
APPFORGE_SOAK_ENVIRONMENT_KIND="$environment_kind" \
APPFORGE_SOAK_OUTPUT="$temporary/report.json" \
python3 <<'PY'
import concurrent.futures
import datetime
import hashlib
import json
import math
import os
import ssl
import statistics
import threading
import time
import urllib.error
import urllib.request

origin = os.environ["APPFORGE_SOAK_API_URL"].rstrip("/")
username = os.environ["APPFORGE_SOAK_ADMIN_USERNAME"]
password_file = os.environ["APPFORGE_SOAK_ADMIN_PASSWORD_FILE"]
duration = int(os.environ["APPFORGE_SOAK_DURATION_SECONDS"])
max_errors = int(os.environ["APPFORGE_SOAK_MAX_ERRORS"])
max_consecutive_errors = int(os.environ["APPFORGE_SOAK_MAX_CONSECUTIVE_ERRORS"])
ca_file = os.environ.get("APPFORGE_SOAK_CA_FILE", "")
ssl_context = ssl.create_default_context(cafile=ca_file or None)
started_at = datetime.datetime.now(datetime.timezone.utc)
started_monotonic = time.monotonic()
deadline = started_monotonic + duration
token_lock = threading.Lock()
token = ""

with open(password_file, encoding="utf-8") as source:
    password = source.read().strip()
if not password:
    raise SystemExit("验收失败: 管理员密码文件为空")

def request_json(path, method="GET", body=None, bearer=None, timeout=5):
    headers = {"Accept": "application/json", "User-Agent": "appforge-v7-api-soak/1"}
    raw = None
    if body is not None:
        raw = json.dumps(body, separators=(",", ":")).encode()
        headers["Content-Type"] = "application/json"
    if bearer:
        headers["Authorization"] = "Bearer " + bearer
    request = urllib.request.Request(origin + path, data=raw, headers=headers, method=method)
    with urllib.request.urlopen(request, timeout=timeout, context=ssl_context) as response:
        payload = response.read(1 << 20)
        if response.status != 200:
            raise RuntimeError("http-status-" + str(response.status))
    return json.loads(payload)

def login():
    global token
    payload = request_json(
        "/admin/system/auth/login", method="POST", body={"username": username, "password": password}, timeout=10
    )
    candidate = payload.get("data", {}).get("token", "")
    if not isinstance(candidate, str) or not candidate:
        raise RuntimeError("login-token-missing")
    with token_lock:
        token = candidate

login()
password = ""

specifications = {
    "healthz": {
        "path": "/healthz",
        "concurrency": int(os.environ["APPFORGE_SOAK_HEALTH_CONCURRENCY"]),
        "interval": int(os.environ["APPFORGE_SOAK_HEALTH_INTERVAL_MS"]) / 1000,
        "p99Limit": float(os.environ["APPFORGE_SOAK_HEALTH_P99_SECONDS"]),
    },
    "readyz": {
        "path": "/readyz",
        "concurrency": int(os.environ["APPFORGE_SOAK_READY_CONCURRENCY"]),
        "interval": int(os.environ["APPFORGE_SOAK_READY_INTERVAL_MS"]) / 1000,
        "p99Limit": float(os.environ["APPFORGE_SOAK_READY_P99_SECONDS"]),
    },
    "deployment": {
        "path": "/admin/core/enterprise/deployment",
        "concurrency": int(os.environ["APPFORGE_SOAK_DEPLOYMENT_CONCURRENCY"]),
        "interval": int(os.environ["APPFORGE_SOAK_DEPLOYMENT_INTERVAL_MS"]) / 1000,
        "p99Limit": float(os.environ["APPFORGE_SOAK_DEPLOYMENT_P99_SECONDS"]),
    },
}
metrics = {
    name: {"latencies": [], "errors": {}, "errorExamples": [], "maxConsecutiveErrors": 0, "workers": []}
    for name in specifications
}
progress_requests = {name: 0 for name in specifications}
metrics_lock = threading.Lock()

def validate(name, payload):
    if name == "healthz" and payload.get("status") != "ok":
        raise RuntimeError("invalid-health-payload")
    if name == "readyz" and payload.get("status") != "ready":
        raise RuntimeError("invalid-ready-payload")
    if name == "deployment":
        if payload.get("code") != 200 or not isinstance(payload.get("data"), dict):
            raise RuntimeError("invalid-deployment-envelope")
        components = payload["data"].get("components", [])
        if len(components) < 3 or any(item.get("status") != "healthy" for item in components):
            raise RuntimeError("unhealthy-deployment-component")

def probe(name):
    specification = specifications[name]
    if name != "deployment":
        return request_json(specification["path"])
    with token_lock:
        bearer = token
    try:
        return request_json(specification["path"], bearer=bearer)
    except urllib.error.HTTPError as error:
        if error.code != 401:
            raise
        login()
        with token_lock:
            bearer = token
        return request_json(specification["path"], bearer=bearer)

def worker(name, worker_index):
    specification = specifications[name]
    local_latencies = []
    local_errors = {}
    local_examples = []
    consecutive = 0
    maximum_consecutive = 0
    while time.monotonic() < deadline:
        cycle_started = time.monotonic()
        try:
            payload = probe(name)
            validate(name, payload)
            local_latencies.append(time.monotonic() - cycle_started)
            consecutive = 0
        except Exception as error:
            error_name = type(error).__name__
            if isinstance(error, urllib.error.HTTPError):
                error_name = "HTTP-" + str(error.code)
            elif isinstance(error, urllib.error.URLError):
                error_name = "URL-" + type(error.reason).__name__
            elif isinstance(error, RuntimeError):
                error_name = str(error)
            local_errors[error_name] = local_errors.get(error_name, 0) + 1
            if len(local_examples) < 5:
                local_examples.append(error_name)
            consecutive += 1
            maximum_consecutive = max(maximum_consecutive, consecutive)
        with metrics_lock:
            progress_requests[name] += 1
        sleep_for = specification["interval"] - (time.monotonic() - cycle_started)
        if sleep_for > 0:
            time.sleep(min(sleep_for, max(0, deadline - time.monotonic())))
    with metrics_lock:
        target = metrics[name]
        target["latencies"].extend(local_latencies)
        for key, value in local_errors.items():
            target["errors"][key] = target["errors"].get(key, 0) + value
        target["errorExamples"].extend(local_examples[: max(0, 20 - len(target["errorExamples"]))])
        target["maxConsecutiveErrors"] = max(target["maxConsecutiveErrors"], maximum_consecutive)
        target["workers"].append(worker_index)

tasks = []
executor = concurrent.futures.ThreadPoolExecutor(
    max_workers=sum(item["concurrency"] for item in specifications.values())
)
for name, specification in specifications.items():
    for worker_index in range(specification["concurrency"]):
        tasks.append(executor.submit(worker, name, worker_index))
while time.monotonic() < deadline:
    time.sleep(min(60, max(0, deadline - time.monotonic())))
    elapsed = int(time.monotonic() - started_monotonic)
    with metrics_lock:
        completed = sum(progress_requests.values())
    print(f"长稳进度: {elapsed}/{duration}s 完整请求={completed}", flush=True)
for task in tasks:
    task.result()
executor.shutdown()

def percentile(values, ratio):
    if not values:
        return 0.0
    ordered = sorted(values)
    return ordered[max(0, math.ceil(len(ordered) * ratio) - 1)]

results = {}
total_errors = 0
for name, specification in specifications.items():
    item = metrics[name]
    latencies = item.pop("latencies")
    errors = sum(item["errors"].values())
    total_errors += errors
    minimum_requests = math.floor(duration / specification["interval"] * specification["concurrency"] * 0.90)
    results[name] = {
        "durationSeconds": duration,
        "concurrency": specification["concurrency"],
        "intervalMilliseconds": int(specification["interval"] * 1000),
        "requests": len(latencies) + errors,
        "successes": len(latencies),
        "errors": errors,
        "errorsByType": item["errors"],
        "errorExamples": item["errorExamples"],
        "maxConsecutiveErrors": item["maxConsecutiveErrors"],
        "minimumRequests": minimum_requests,
        "averageSeconds": statistics.fmean(latencies) if latencies else 0.0,
        "p50Seconds": percentile(latencies, 0.50),
        "p95Seconds": percentile(latencies, 0.95),
        "p99Seconds": percentile(latencies, 0.99),
        "maxSeconds": max(latencies) if latencies else 0.0,
        "p99LimitSeconds": specification["p99Limit"],
    }

actual_duration = time.monotonic() - started_monotonic
run_class = "day" if actual_duration >= 86400 else "hour" if actual_duration >= 3600 else "smoke"
passed = total_errors <= max_errors
for result in results.values():
    passed = passed and result["requests"] >= result["minimumRequests"]
    passed = passed and result["maxConsecutiveErrors"] <= max_consecutive_errors
    passed = passed and result["p99Seconds"] <= result["p99LimitSeconds"]
evidence = {
    "schemaVersion": 1,
    "evidenceType": "v7-authenticated-api-soak",
    "acceptedAt": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds"),
    "result": "passed" if passed else "failed",
    "runClass": run_class,
    "qualifiesHourLevel": actual_duration >= 3600,
    "qualifiesDayLevel": actual_duration >= 86400,
    "environmentKind": os.environ.get("APPFORGE_SOAK_ENVIRONMENT_KIND", "fixture"),
    "targetOriginSha256": hashlib.sha256(origin.lower().encode()).hexdigest(),
    "startedAt": started_at.isoformat(timespec="seconds"),
    "actualDurationSeconds": actual_duration,
    "errorBudget": {"maximumErrors": max_errors, "maximumConsecutiveErrors": max_consecutive_errors},
    "probes": results,
    "verified": [
        "process-liveness-healthz",
        "license-readiness-readyz",
        "jwt-rbac-rpc-database-deployment-status",
        "response-payload-semantics",
        "bounded-latency-and-error-budget",
    ],
    "limitations": [
        "synthetic-environment" if os.environ.get("APPFORGE_SOAK_ENVIRONMENT_KIND", "fixture") == "fixture" else "customer-declared-test-environment",
        "does-not-exercise-apk-build-or-object-throughput",
        "does-not-replace-day-level-or-customer-peak-capacity-unless-explicitly-qualified",
    ],
}
with open(os.environ["APPFORGE_SOAK_OUTPUT"], "w", encoding="utf-8") as output:
    json.dump(evidence, output, ensure_ascii=False, indent=2)
    output.write("\n")
PY

install -m 0600 "$temporary/report.json" "$report_file"
if ! jq -e '.result == "passed"' "$report_file" >/dev/null; then
  echo "验收失败: API 长稳错误预算、请求数或 P99 门禁未通过；证据: $report_file" >&2
  exit 1
fi
echo "通过: API 长稳时长=${duration_seconds}s 分类=$(jq -r .runClass "$report_file")"
echo "证据: $report_file"
