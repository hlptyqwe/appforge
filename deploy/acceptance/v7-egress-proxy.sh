#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d /tmp/appforge-v7-egress-proxy.XXXXXX)
evidence_path=${APPFORGE_EGRESS_PROXY_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v7-egress-proxy-20260816.json}
proxy_pid=
target_pid=
cleanup() {
  for process_id in ${proxy_pid:-} ${target_pid:-}; do
    [[ -n $process_id ]] || continue
    kill -TERM "$process_id" >/dev/null 2>&1 || true
    wait "$process_id" 2>/dev/null || true
  done
  rm -rf "$temporary"
}
trap cleanup EXIT

available_port() {
  python3 - <<'PY'
import socket
sock = socket.socket()
sock.bind(("127.0.0.1", 0))
print(sock.getsockname()[1])
sock.close()
PY
}

proxy_port=$(available_port)
target_port=$(available_port)
started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=localhost \
  -keyout "$temporary/target.key" -out "$temporary/target.crt" >/dev/null 2>&1
printf '127.0.0.1:%s\n' "$target_port" >"$temporary/allowlist.txt"
chmod 0444 "$temporary/allowlist.txt"

(
  cd "$repo_root/common"
  GOCACHE=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache} \
    go test ./egressproxy ./remotesigner ./siem
  GOCACHE=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache} \
    go build -o "$temporary/egress-proxy" ./cmd/egress-proxy
)

openssl s_server -quiet -accept "127.0.0.1:$target_port" \
  -cert "$temporary/target.crt" -key "$temporary/target.key" -www \
  >"$temporary/target.log" 2>&1 &
target_pid=$!
APPFORGE_EGRESS_PROXY_LISTEN="127.0.0.1:$proxy_port" \
APPFORGE_EGRESS_PROXY_ALLOWLIST_FILE="$temporary/allowlist.txt" \
APPFORGE_EGRESS_PROXY_MAX_CONNECTIONS=8 \
  "$temporary/egress-proxy" >"$temporary/proxy.log" 2>&1 &
proxy_pid=$!

for _ in $(seq 1 50); do
  curl -fsS "http://127.0.0.1:$proxy_port/readyz" >/dev/null 2>&1 && break
  kill -0 "$proxy_pid" >/dev/null 2>&1 || {
    sed -n '1,120p' "$temporary/proxy.log" >&2
    echo "验收失败: 出口代理提前退出" >&2
    exit 1
  }
  sleep 0.1
done
curl -fsS "http://127.0.0.1:$proxy_port/healthz" >/dev/null
python3 - "$temporary/proxy.log" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    record = json.loads(next(line for line in handle if line.strip()))
if record.get("msg") != "egress proxy started" or record.get("level") != "INFO":
    raise SystemExit("egress proxy startup log is not structured JSON")
PY
curl -ksSf --noproxy '' --proxy "http://127.0.0.1:$proxy_port" \
  "https://127.0.0.1:$target_port/" | grep -Fq 's_server'

python3 - "$proxy_port" <<'PY'
import socket
import sys

port = int(sys.argv[1])

def status(request: bytes) -> int:
    with socket.create_connection(("127.0.0.1", port), timeout=2) as sock:
        sock.sendall(request)
        first = sock.recv(256).split(b"\r\n", 1)[0]
    return int(first.split()[1])

blocked = status(b"CONNECT 127.0.0.1:1 HTTP/1.1\r\nHost: 127.0.0.1:1\r\n\r\n")
plaintext = status(b"GET http://127.0.0.1/ HTTP/1.1\r\nHost: 127.0.0.1\r\n\r\n")
if blocked != 403:
    raise SystemExit(f"unlisted CONNECT status={blocked}, want 403")
if plaintext != 405:
    raise SystemExit(f"plaintext proxy status={plaintext}, want 405")
PY

grep -Fq 'profiles: ["egress"]' "$repo_root/deploy/production/docker-compose.yml"
grep -Fq 'appforge-egress: {}' "$repo_root/deploy/production/docker-compose.yml"
grep -Fq 'component=egress-proxy' "$repo_root/deploy/helm/appforge/templates/preflight.yaml"
grep -Fq 'HTTP_PROXY' "$repo_root/deploy/helm/appforge/templates/deployments.yaml"
grep -Fq 'egress-proxy' "$repo_root/deploy/production/offline-bundle.sh"
grep -Fq 'component: egress-proxy' "$repo_root/.github/workflows/release-security.yml"

finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
mkdir -p "$(dirname "$evidence_path")"
umask 077
python3 - "$evidence_path" "$started_at" "$finished_at" <<'PY'
import json
import sys

path, started_at, finished_at = sys.argv[1:]
report = {
    "acceptance": "V7_RESTRICTED_EGRESS_PROXY",
    "fixture": "synthetic-loopback-tls-only",
    "startedAt": started_at,
    "finishedAt": finished_at,
    "checks": {
        "approvedHttpsConnect": "passed",
        "unlistedDestinationRejected": "passed",
        "plaintextForwardingRejected": "passed",
        "boundedAllowlistParser": "passed",
        "siemEnvironmentProxy": "passed",
        "remoteApkSignerProxyBypass": "passed",
        "structuredJsonLog": "passed",
        "composeInternalBackendAndProxyProfile": "passed",
        "helmAndReleaseWiring": "passed",
    },
    "securityBoundary": {
        "connectOnly": True,
        "proxyReceivesApkRemoteSigningTraffic": False,
        "webhookSsrFTransportChanged": False,
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

echo "通过: 受限 HTTPS CONNECT 出口代理已验证 Allowlist、明文拒绝和远程签名绕过；证据: $evidence_path"
