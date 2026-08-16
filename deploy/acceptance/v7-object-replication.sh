#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d)
delivery="$temporary/production"
project="appforge-v7-object-dr-$$"
report_file=${APPFORGE_OBJECT_DR_REPORT_FILE:-}
[[ $project =~ ^appforge-v7-object-dr-[0-9]+$ ]]

now_millis() {
  python3 -c 'import time; print(time.time_ns() // 1_000_000)'
}

cleanup() {
  set +e
  if [[ -f $delivery/docker-compose.yml && -f $delivery/.env ]]; then
    docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml" down -v --remove-orphans >/dev/null 2>&1
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT

mkdir -p "$delivery" "$temporary/fixtures"
cp "$repo_root/deploy/production/configure-object-replication.sh" "$delivery/configure-object-replication.sh"
chmod 0755 "$delivery/configure-object-replication.sh"
printf '%s' 'synthetic-object-version-one' >"$temporary/fixtures/object-v1.apk"
printf '%s' 'synthetic-object-version-two-recovered' >"$temporary/fixtures/object-v2.apk"
expected_v1_sha=$(sha256sum "$temporary/fixtures/object-v1.apk" | cut -d ' ' -f 1)
expected_v2_sha=$(sha256sum "$temporary/fixtures/object-v2.apk" | cut -d ' ' -f 1)

cat >"$delivery/docker-compose.yml" <<'YAML'
services:
  minio:
    image: minio/minio:RELEASE.2025-04-22T22-12-26Z
    command: ["server", "/data", "--address", ":9000"]
    environment:
      MINIO_ROOT_USER: source-access-key
      MINIO_ROOT_PASSWORD: source-secret-password-1234
    volumes: [source-data:/data]
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://127.0.0.1:9000/minio/health/live"]
      interval: 2s
      timeout: 2s
      retries: 60
  replica:
    image: minio/minio:RELEASE.2025-04-22T22-12-26Z
    command: ["server", "/data", "--address", ":9000"]
    environment:
      MINIO_ROOT_USER: replica-access-key
      MINIO_ROOT_PASSWORD: replica-secret-password-123
    volumes: [replica-data:/data]
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://127.0.0.1:9000/minio/health/live"]
      interval: 2s
      timeout: 2s
      retries: 60
  minio-init:
    image: minio/mc:RELEASE.2025-04-16T18-13-26Z
    profiles: [tools]
    environment:
      MC_CONFIG_DIR: /tmp/mc
      MINIO_ACCESS_KEY: source-access-key
      MINIO_SECRET_KEY: source-secret-password-1234
      REPLICA_ENDPOINT: http://replica:9000
      REPLICA_ACCESS_KEY: replica-access-key
      REPLICA_SECRET_KEY: replica-secret-password-123
      REPLICA_BUCKET: appforge
      REPLICA_RULE_ID: appforge-dr
      REPLICA_SYNC: "true"
    tmpfs: [/tmp]
volumes:
  source-data:
  replica-data:
YAML

cat >"$delivery/.env" <<EOF
COMPOSE_PROJECT_NAME=$project
APPFORGE_MINIO_ACCESS_KEY=source-access-key
APPFORGE_MINIO_SECRET_KEY=source-secret-password-1234
APPFORGE_REPLICA_ENDPOINT=http://replica:9000
APPFORGE_REPLICA_ACCESS_KEY=replica-access-key
APPFORGE_REPLICA_SECRET_KEY=replica-secret-password-123
APPFORGE_REPLICA_BUCKET=appforge
APPFORGE_REPLICA_RULE_ID=appforge-dr
APPFORGE_REPLICA_SYNC=true
APPFORGE_ALLOW_INSECURE_REPLICA=true
EOF
chmod 0600 "$delivery/.env"

compose=(docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml")
"${compose[@]}" up -d --wait minio replica >"$temporary/start.log" 2>&1 || {
  tail -100 "$temporary/start.log" >&2
  exit 1
}
APPFORGE_ENV_FILE="$delivery/.env" "$delivery/configure-object-replication.sh" >"$temporary/configure.log" 2>&1 || {
  tail -160 "$temporary/configure.log" >&2
  exit 1
}

mc_source() {
  "${compose[@]}" run --rm -T --no-deps --entrypoint /bin/sh \
    -v "$temporary/fixtures:/fixtures:ro" minio-init -c \
    'set -eu; mc alias set source http://minio:9000 "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null; exec mc "$@"' \
    sh "$@"
}

mc_replica() {
  "${compose[@]}" run --rm -T --no-deps --entrypoint /bin/sh minio-init -c \
    'set -eu; mc alias set replica "$REPLICA_ENDPOINT" "$REPLICA_ACCESS_KEY" "$REPLICA_SECRET_KEY" >/dev/null; exec mc "$@"' \
    sh "$@"
}

replica_sha() {
  mc_replica cat replica/appforge/tenants/970001/builds/synthetic.apk 2>/dev/null | sha256sum | cut -d ' ' -f 1
}

wait_for_replica_sha() {
  local expected=$1
  for _ in $(seq 1 60); do
    if [[ $(replica_sha 2>/dev/null || true) == "$expected" ]]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

first_started_ms=$(now_millis)
mc_source cp /fixtures/object-v1.apk source/appforge/tenants/970001/builds/synthetic.apk >/dev/null
wait_for_replica_sha "$expected_v1_sha" || { echo "验收失败: 首版本未复制到副本" >&2; exit 1; }
first_replicated_ms=$(now_millis)

second_started_ms=$(now_millis)
mc_source cp /fixtures/object-v2.apk source/appforge/tenants/970001/builds/synthetic.apk >/dev/null
wait_for_replica_sha "$expected_v2_sha" || { echo "验收失败: 新版本未复制到副本" >&2; exit 1; }
second_replicated_ms=$(now_millis)

version_output=$(mc_replica stat --versions --json replica/appforge/tenants/970001/builds/synthetic.apk)
version_count=$(printf '%s\n' "$version_output" | python3 -c 'import json,sys; print(sum(1 for line in sys.stdin if line.strip() and json.loads(line).get("versionID")))')
((version_count >= 2)) || { echo "验收失败: 副本未保留两个对象版本" >&2; exit 1; }

mc_source rm source/appforge/tenants/970001/builds/synthetic.apk >/dev/null
for _ in $(seq 1 60); do
  if ! mc_replica stat replica/appforge/tenants/970001/builds/synthetic.apk >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if mc_replica stat replica/appforge/tenants/970001/builds/synthetic.apk >/dev/null 2>&1; then
  echo "验收失败: 删除标记未复制到副本" >&2
  exit 1
fi

recovery_started_ms=$(now_millis)
"${compose[@]}" stop minio >/dev/null
"${compose[@]}" rm -f minio >/dev/null
docker volume rm "${project}_source-data" >/dev/null
mc_replica undo --action DELETE replica/appforge/tenants/970001/builds/synthetic.apk >/dev/null
[[ $(replica_sha) == "$expected_v2_sha" ]] || { echo "验收失败: 源端销毁后副本无法恢复最新版本" >&2; exit 1; }
recovery_verified_ms=$(now_millis)

first_replication_seconds=$(((first_replicated_ms - first_started_ms + 999) / 1000))
second_replication_seconds=$(((second_replicated_ms - second_started_ms + 999) / 1000))
recovery_seconds=$(((recovery_verified_ms - recovery_started_ms + 999) / 1000))

"${compose[@]}" down -v --remove-orphans >/dev/null
containers_left=$(docker ps -aq --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
volumes_left=$(docker volume ls -q --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
networks_left=$(docker network ls -q --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
[[ $containers_left == 0 && $volumes_left == 0 && $networks_left == 0 ]]

if [[ -n $report_file ]]; then
  mkdir -p "$(dirname "$report_file")"
  umask 077
  APPFORGE_OBJECT_DR_FIRST_SECONDS="$first_replication_seconds" \
    APPFORGE_OBJECT_DR_SECOND_SECONDS="$second_replication_seconds" \
    APPFORGE_OBJECT_DR_RECOVERY_SECONDS="$recovery_seconds" \
    APPFORGE_OBJECT_DR_VERSION_COUNT="$version_count" \
    APPFORGE_OBJECT_DR_V1_SHA="$expected_v1_sha" \
    APPFORGE_OBJECT_DR_V2_SHA="$expected_v2_sha" \
    APPFORGE_OBJECT_DR_CONTAINERS_LEFT="$containers_left" \
    APPFORGE_OBJECT_DR_VOLUMES_LEFT="$volumes_left" \
    APPFORGE_OBJECT_DR_NETWORKS_LEFT="$networks_left" \
    python3 -c '
import json, os, sys
integer=lambda key: int(os.environ[key])
json.dump({
  "schemaVersion": 1,
  "scenario": "isolated-synthetic-object-replication-recovery",
  "acceptanceScript": "deploy/acceptance/v7-object-replication.sh",
  "replicationMode": "server-side-sync",
  "versioning": True,
  "replicatedVersionCount": integer("APPFORGE_OBJECT_DR_VERSION_COUNT"),
  "firstReplicationSeconds": integer("APPFORGE_OBJECT_DR_FIRST_SECONDS"),
  "secondReplicationSeconds": integer("APPFORGE_OBJECT_DR_SECOND_SECONDS"),
  "sourceDestroyed": True,
  "deleteMarkerReplayed": True,
  "recoverySeconds": integer("APPFORGE_OBJECT_DR_RECOVERY_SECONDS"),
  "verifiedSha256": [os.environ["APPFORGE_OBJECT_DR_V1_SHA"], os.environ["APPFORGE_OBJECT_DR_V2_SHA"]],
  "residualResources": {
    "containers": integer("APPFORGE_OBJECT_DR_CONTAINERS_LEFT"),
    "volumes": integer("APPFORGE_OBJECT_DR_VOLUMES_LEFT"),
    "networks": integer("APPFORGE_OBJECT_DR_NETWORKS_LEFT")
  },
  "dataPolicy": "synthetic-only",
  "limitations": ["same-docker-host", "not-customer-cross-region-network", "not-production-object-volume"]
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
' >"$report_file"
  chmod 0600 "$report_file"
fi

echo "通过: 合成对象双版本复制、SHA核对、删除标记复制、源卷销毁及副本版本恢复；复制=${first_replication_seconds}s/${second_replication_seconds}s，恢复=${recovery_seconds}s，残留资源=0"
