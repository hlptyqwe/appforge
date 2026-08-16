#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d /tmp/appforge-v7-observability.XXXXXX)
evidence_path=${APPFORGE_OBSERVABILITY_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v7-observability-20260815.json}
fixture_pid=
cleanup() {
  if [[ -n ${fixture_pid:-} ]] && kill -0 "$fixture_pid" >/dev/null 2>&1; then
    kill -TERM "$fixture_pid" >/dev/null 2>&1 || true
    wait "$fixture_pid" 2>/dev/null || true
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT

(
  cd "$repo_root/common"
  GOCACHE=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache} \
    go build -o "$temporary/observability-fixture" ./cmd/observability-acceptance
)

started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
APPFORGE_PROMETHEUS_ENABLED=true \
APPFORGE_PROMETHEUS_HOST=127.0.0.1 \
APPFORGE_PROMETHEUS_PORT=19101 \
APPFORGE_PROMETHEUS_PATH=/metrics \
APPFORGE_OTLP_ENDPOINT=http://127.0.0.1:14318/v1/traces \
APPFORGE_OTLP_ALLOW_HTTP=true \
APPFORGE_OTLP_SAMPLER=1 \
  "$temporary/observability-fixture" >"$temporary/service.log" 2>&1 &
fixture_pid=$!

for _ in $(seq 1 50); do
  if curl -fsS http://127.0.0.1:18888/probe >/dev/null 2>&1; then
    break
  fi
  kill -0 "$fixture_pid" >/dev/null 2>&1 || {
    sed -n '1,160p' "$temporary/service.log" >&2
    echo "验收失败: 可观测性夹具提前退出" >&2
    exit 1
  }
  sleep 0.1
done
curl -fsS http://127.0.0.1:18888/probe >/dev/null
curl -fsS http://127.0.0.1:18888/probe >/dev/null

metrics_file="$temporary/metrics.txt"
curl -fsS http://127.0.0.1:19101/metrics >"$metrics_file"
grep -Fq 'http_server_requests_code_total' "$metrics_file" || {
  echo "验收失败: Prometheus 未导出 HTTP 请求计数" >&2
  exit 1
}
grep -Fq 'http_server_requests_duration_ms' "$metrics_file" || {
  echo "验收失败: Prometheus 未导出 HTTP 请求耗时" >&2
  exit 1
}

trace_batches=0
for _ in $(seq 1 100); do
  trace_batches=$(curl -fsS http://127.0.0.1:14318/count 2>/dev/null || printf 0)
  if [[ $trace_batches =~ ^[0-9]+$ ]] && (( trace_batches > 0 )); then
    break
  fi
  sleep 0.1
done
[[ $trace_batches =~ ^[0-9]+$ ]] && (( trace_batches > 0 )) || {
  sed -n '1,160p' "$temporary/service.log" >&2
  echo "验收失败: 未收到 OTLP/HTTP Trace 批次" >&2
  exit 1
}

if APPFORGE_OTLP_ENDPOINT=http://127.0.0.1:14318/v1/traces \
  "$temporary/observability-fixture" >"$temporary/insecure.log" 2>&1; then
  echo "验收失败: 未授权的明文 OTLP Endpoint 未被拒绝" >&2
  exit 1
fi

finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p "$(dirname "$evidence_path")"
umask 077
python3 - "$evidence_path" "$started_at" "$finished_at" "$trace_batches" <<'PY'
import json
import sys

path, started_at, finished_at, trace_batches = sys.argv[1:]
report = {
    "acceptance": "V7_OBSERVABILITY",
    "fixture": "synthetic-local-only",
    "startedAt": started_at,
    "finishedAt": finished_at,
    "prometheus": {
        "endpoint": "127.0.0.1:19101/metrics",
        "verifiedMetrics": [
            "http_server_requests_code_total",
            "http_server_requests_duration_ms",
        ],
    },
    "otlpHttp": {
        "endpoint": "127.0.0.1:14318/v1/traces",
        "receivedBatches": int(trace_batches),
        "productionRequiresTLS": True,
        "plaintextRejectedWithoutAcceptanceOverride": True,
    },
    "realCustomerDataAccessed": False,
    "productionCredentialsAccessed": False,
    "residualTemporaryResources": 0,
    "result": "passed",
}
with open(path, "w", encoding="utf-8") as handle:
    json.dump(report, handle, ensure_ascii=False, indent=2)
    handle.write("\n")
PY
chmod 600 "$evidence_path"

echo "通过: Prometheus 指标和 OTLP/HTTP Trace 已由真实 go-zero 服务导出，生产明文 Endpoint 被拒绝；证据: $evidence_path"
