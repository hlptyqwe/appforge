#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
minio_image=${APPFORGE_CUSTOMER_PROBE_MINIO_IMAGE:-minio/minio:RELEASE.2025-04-22T22-12-26Z}
mc_image=${APPFORGE_CUSTOMER_PROBE_MC_IMAGE:-minio/mc:RELEASE.2025-04-16T18-13-26Z}
report_file=${APPFORGE_CUSTOMER_PROBE_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-customer-storage-probe-fixture-20260817.json}
size_bytes=${APPFORGE_CUSTOMER_PROBE_SIZE_BYTES:-1048576}
expected_agent_version=${APPFORGE_EXPECTED_LOCAL_AGENT_VERSION:-}
suffix="$(date +%s)-$$"
network="appforge-v7-storage-probe-$suffix"
minio_container="appforge-v7-storage-probe-minio-$suffix"
state_volume="appforge-v7-storage-probe-state-$suffix"
secret_volume="appforge-v7-storage-probe-secret-$suffix"
bucket="appforge-probe-$suffix"
prefix="tenants/970001/agents/storage-probe-$suffix"
temporary=$(mktemp -d /tmp/appforge-v7-storage-probe.XXXXXX)

cleanup() {
  set +e
  if [[ $(docker inspect -f '{{index .Config.Labels "appforge.acceptance"}}' "$minio_container" 2>/dev/null || true) == v7-customer-storage-probe ]]; then
    docker rm -f "$minio_container" >/dev/null 2>&1
  fi
  if [[ $(docker network inspect -f '{{index .Labels "appforge.acceptance"}}' "$network" 2>/dev/null || true) == v7-customer-storage-probe ]]; then
    docker network rm "$network" >/dev/null 2>&1
  fi
  for volume in "$state_volume" "$secret_volume"; do
    if [[ $(docker volume inspect -f '{{index .Labels "appforge.acceptance"}}' "$volume" 2>/dev/null || true) == v7-customer-storage-probe ]]; then
      docker volume rm "$volume" >/dev/null 2>&1
    fi
  done
  rm -rf "$temporary"
}
trap cleanup EXIT

for command_name in docker openssl python3; do
  command -v "$command_name" >/dev/null || {
    echo "验收失败: 缺少命令 $command_name" >&2
    exit 1
  }
done
root_access="probe$(openssl rand -hex 8)"
root_secret="probe$(openssl rand -hex 24)"
[[ $size_bytes =~ ^[1-9][0-9]*$ ]] && ((size_bytes >= 1024 && size_bytes <= 67108864)) || {
  echo "验收失败: 探针大小必须在 1024 到 67108864 字节之间" >&2
  exit 1
}
for image in "$agent_image" "$minio_image" "$mc_image"; do
  docker image inspect "$image" >/dev/null || {
    echo "验收失败: 缺少镜像 $image" >&2
    exit 1
  }
done
agent_version=$(docker run --rm "$agent_image" version | awk '{print $2}')
[[ $agent_version =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]] || {
  echo "验收失败: Local Agent 未报告语义化版本" >&2
  exit 1
}
if [[ -n $expected_agent_version && $agent_version != "$expected_agent_version" ]]; then
  echo "验收失败: Local Agent 版本=$agent_version，期望=$expected_agent_version" >&2
  exit 1
fi

openssl ecparam -name prime256v1 -genkey -noout -out "$temporary/ca.key"
openssl req -x509 -new -sha256 -key "$temporary/ca.key" -days 2 -subj '/CN=AppForge V7 Storage Probe CA' -out "$temporary/ca.crt"
openssl ecparam -name prime256v1 -genkey -noout -out "$temporary/agent.key"
openssl req -new -sha256 -key "$temporary/agent.key" -subj '/CN=storage-probe-agent' -out "$temporary/agent.csr"
openssl x509 -req -sha256 -in "$temporary/agent.csr" -CA "$temporary/ca.crt" -CAkey "$temporary/ca.key" \
  -CAcreateserial -days 1 -out "$temporary/agent.crt" >/dev/null 2>&1

STATE_FILE="$temporary/state.json" APPFORGE_PROBE_PREFIX="$prefix" \
APPFORGE_PROBE_AGENT_VERSION="$agent_version" python3 - <<'PY'
import json
import os

payload = {
    "agentId": 970001,
    "gatewayUrl": "https://gateway.invalid.example",
    "certificate": "/state/agent.crt",
    "privateKey": "/state/agent.key",
    "clientCa": "/state/ca.crt",
    "gatewayCa": "/state/ca.crt",
    "customerStorageRef": "local-file:///customer-storage.json#" + os.environ["APPFORGE_PROBE_PREFIX"],
    "protocol": 3,
    "agentVersion": os.environ["APPFORGE_PROBE_AGENT_VERSION"],
    "lastTimestamp": 0,
}
with open(os.environ["STATE_FILE"], "w", encoding="utf-8") as handle:
    json.dump(payload, handle, separators=(",", ":"))
PY

SECRET_FILE="$temporary/customer-storage.json" APPFORGE_PROBE_MINIO_CONTAINER="$minio_container" \
APPFORGE_PROBE_ACCESS="$root_access" APPFORGE_PROBE_SECRET="$root_secret" \
APPFORGE_PROBE_BUCKET="$bucket" APPFORGE_PROBE_PREFIX="$prefix" python3 - <<'PY'
import json
import os

payload = {
    "provider": "minio",
    "endpoint": "http://" + os.environ["APPFORGE_PROBE_MINIO_CONTAINER"] + ":9000",
    "region": "us-east-1",
    "access_key_id": os.environ["APPFORGE_PROBE_ACCESS"],
    "access_key_secret": os.environ["APPFORGE_PROBE_SECRET"],
    "bucket": os.environ["APPFORGE_PROBE_BUCKET"],
    "prefix": os.environ["APPFORGE_PROBE_PREFIX"],
}
with open(os.environ["SECRET_FILE"], "w", encoding="utf-8") as handle:
    json.dump(payload, handle, separators=(",", ":"))
PY
chmod 0600 "$temporary/state.json" "$temporary/agent.crt" "$temporary/agent.key" "$temporary/ca.crt" "$temporary/customer-storage.json"

docker network create --label appforge.acceptance=v7-customer-storage-probe "$network" >/dev/null
docker volume create --label appforge.acceptance=v7-customer-storage-probe "$state_volume" >/dev/null
docker volume create --label appforge.acceptance=v7-customer-storage-probe "$secret_volume" >/dev/null
docker run -d --name "$minio_container" --network "$network" \
  --label appforge.acceptance=v7-customer-storage-probe \
  -e MINIO_ROOT_USER="$root_access" -e MINIO_ROOT_PASSWORD="$root_secret" \
  "$minio_image" server /data >/dev/null

for _ in $(seq 1 60); do
  if docker run --rm --network "$network" --entrypoint /bin/sh \
    -e MC_HOST_customer="http://$root_access:$root_secret@$minio_container:9000" \
    "$mc_image" -ec 'mc ready customer >/dev/null'; then
    break
  fi
  sleep 1
done
docker run --rm --network "$network" --entrypoint /bin/sh \
  -e MC_HOST_customer="http://$root_access:$root_secret@$minio_container:9000" \
  -e BUCKET="$bucket" -e PREFIX="$prefix" "$mc_image" -ec '
    mc mb "customer/$BUCKET" >/dev/null
    printf %s existing-customer-marker | mc pipe "customer/$BUCKET/$PREFIX/existing/marker.txt" >/dev/null
  '

docker run --rm --user 0 -v "$state_volume:/state" -v "$temporary:/fixture:ro" \
  --entrypoint /bin/sh "$agent_image" -ec '
    cp /fixture/state.json /fixture/agent.crt /fixture/agent.key /fixture/ca.crt /state/
    chown 65532:65532 /state/state.json /state/agent.crt /state/agent.key /state/ca.crt
    chmod 0600 /state/state.json /state/agent.crt /state/agent.key /state/ca.crt
  '
docker run --rm --user 0 -v "$secret_volume:/secrets" -v "$temporary:/fixture:ro" \
  --entrypoint /bin/sh "$agent_image" -ec '
    cp /fixture/customer-storage.json /secrets/customer-storage.json
    chown 65532:65532 /secrets/customer-storage.json
    chmod 0600 /secrets/customer-storage.json
  '

docker run --rm --network "$network" -v "$state_volume:/state:ro" -v "$secret_volume:/secrets:ro" \
  "$agent_image" customer-storage-probe --state-dir /state --secret-root /secrets --report - \
  --environment-kind fixture --size-bytes "$size_bytes" --timeout 2m \
  --confirm-synthetic-only SYNTHETIC_WRITE_READ_DELETE >"$temporary/report.json"

REPORT_FILE="$temporary/report.json" EXPECTED_SIZE="$size_bytes" EXPECTED_AGENT_VERSION="$agent_version" python3 - <<'PY'
import json
import os

with open(os.environ["REPORT_FILE"], encoding="utf-8") as handle:
    payload = json.load(handle)
assert payload["schemaVersion"] == 1
assert payload["evidenceType"] == "v7-customer-object-storage-site-probe"
assert payload["result"] == "passed"
assert payload["environmentKind"] == "fixture"
assert payload["provider"] == "minio"
assert payload["runtime"]["agentVersion"] == os.environ["EXPECTED_AGENT_VERSION"]
assert payload["probe"]["objectSizeBytes"] == int(os.environ["EXPECTED_SIZE"])
assert payload["probe"]["deleteConfirmed"] is True
assert payload["probe"]["existingObjectsRead"] == 0
assert payload["probe"]["bucketOrPolicyMutationRun"] is False
assert "temporary-provider-fixture-not-customer-environment" in payload["limitations"]
PY
for forbidden in "$root_access" "$root_secret" "$minio_container" "$bucket" "$prefix"; do
  if grep -Fq "$forbidden" "$temporary/report.json"; then
    echo "验收失败: 机器证据泄漏客户目标或凭据" >&2
    exit 1
  fi
done

remaining_probe_objects=$(docker run --rm --network "$network" --entrypoint /bin/sh \
  -e MC_HOST_customer="http://$root_access:$root_secret@$minio_container:9000" \
  -e BUCKET="$bucket" -e PREFIX="$prefix" "$mc_image" -ec '
    mc find "customer/$BUCKET/$PREFIX/acceptance/storage-probe" --type f 2>/dev/null | wc -l
  ' | tr -d ' ')
[[ $remaining_probe_objects == 0 ]] || {
  echo "验收失败: 探针合成对象未清理" >&2
  exit 1
}
existing_marker=$(docker run --rm --network "$network" --entrypoint /bin/sh \
  -e MC_HOST_customer="http://$root_access:$root_secret@$minio_container:9000" \
  -e BUCKET="$bucket" -e PREFIX="$prefix" "$mc_image" -ec \
  'mc cat "customer/$BUCKET/$PREFIX/existing/marker.txt"')
[[ $existing_marker == existing-customer-marker ]] || {
  echo "验收失败: 探针修改了既有对象" >&2
  exit 1
}

mkdir -p "$(dirname "$report_file")"
cp "$temporary/report.json" "$report_file"
chmod 0600 "$report_file"
echo "通过: Local Agent 客户对象存储现场探针真实 MinIO Provider 路径、完整回读、删除确认、既有对象不变和证据脱敏"
echo "证据: $report_file"
