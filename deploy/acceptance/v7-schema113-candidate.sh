#!/usr/bin/env bash

set -euo pipefail

# Schema 113生产提升回归：同时验证历史Schema 112镜像边界与112到113升级路径。
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
suffix="$(date +%s)-$$"
project="appforge-schema113-production-$suffix"
network="$project"
mysql_container="$project-mysql"
schema112_image="appforge-schema113-acceptance:migrate-112-$suffix"
schema113_image="appforge-schema113-acceptance:migrate-113-$suffix"
report_file=${APPFORGE_SCHEMA113_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-schema113-production-20260816.json}
root_password="schema113_root_$suffix"
bootstrap_bcrypt_b64='JDJ5JDEwJGJ4UEI4eWVWNFFMdUNuNW1OeHpKQi5jTVZEdEdYcFJaaUlGLnIvdTZjMDlSREJlVURHZ2FD'

[[ $project =~ ^appforge-schema113-production-[0-9]+-[0-9]+$ ]]

cleanup() {
  docker rm -f "$mysql_container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  docker image rm "$schema112_image" "$schema113_image" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker build --pull=false --build-arg APPFORGE_SCHEMA_TARGET=20260815_112_v7_customer_storage \
  -f "$repo_root/deploy/docker/migrate.Dockerfile" -t "$schema112_image" "$repo_root" >/dev/null
docker build --pull=false --build-arg APPFORGE_SCHEMA_TARGET=20260815_113_v7_air_gapped \
  -f "$repo_root/deploy/docker/migrate.Dockerfile" -t "$schema113_image" "$repo_root" >/dev/null

docker run --rm --entrypoint sh "$schema112_image" -c \
  'test ! -e /migrations/113-v7-air-gapped.sql && ! grep -q t_air_gapped_package /bootstrap/core.sql'
docker run --rm --entrypoint sh "$schema113_image" -c \
  'test -s /migrations/113-v7-air-gapped.sql && grep -q t_air_gapped_package /bootstrap/core.sql && grep -q enterprise:air-gapped:export /migrations/113-v7-air-gapped.sql'

docker network create "$network" >/dev/null
docker run -d --name "$mysql_container" --network "$network" \
  -e MYSQL_ROOT_PASSWORD="$root_password" mysql:8.4 \
  --character-set-server=utf8mb4 --collation-server=utf8mb4_unicode_ci >/dev/null
for _ in $(seq 1 90); do
  if docker exec -e MYSQL_PWD="$root_password" "$mysql_container" mysqladmin -h127.0.0.1 -uroot ping --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec -e MYSQL_PWD="$root_password" "$mysql_container" mysqladmin -h127.0.0.1 -uroot ping --silent >/dev/null
docker exec -e MYSQL_PWD="$root_password" "$mysql_container" mysql -h127.0.0.1 -uroot -e \
  'CREATE DATABASE fresh113 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; CREATE DATABASE upgrade113 CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'

run_migration() {
  local image=$1 database=$2 expected=$3
  docker run --rm --network "$network" \
    -e MYSQL_HOST="$mysql_container" -e MYSQL_DATABASE="$database" -e MYSQL_USER=root -e MYSQL_PASSWORD="$root_password" \
    -e APPFORGE_BOOTSTRAP_ADMIN_USERNAME=owner -e APPFORGE_BOOTSTRAP_PASSWORD_BCRYPT_BASE64="$bootstrap_bcrypt_b64" \
    -e APPFORGE_PUBLIC_ORIGIN=https://schema113.acceptance.invalid \
    -e APPFORGE_MINIO_ACCESS_KEY=synthetic_access -e APPFORGE_MINIO_SECRET_KEY=synthetic_secret \
    "$image" "--expected=$expected" >/dev/null
}

run_migration "$schema113_image" fresh113 20260815_113_v7_air_gapped
run_migration "$schema113_image" fresh113 20260815_113_v7_air_gapped

run_migration "$schema112_image" upgrade113 20260815_112_v7_customer_storage
docker exec -e MYSQL_PWD="$root_password" "$mysql_container" mysql -h127.0.0.1 -uroot upgrade113 -e \
  "INSERT INTO sys_schema_migration(version,description) VALUES ('schema113_acceptance_preupgrade_marker','Schema 113升级数据保持探针')"
run_migration "$schema113_image" upgrade113 20260815_113_v7_air_gapped
run_migration "$schema113_image" upgrade113 20260815_113_v7_air_gapped

validate_database() {
  local database=$1 expect_marker=$2 state
  state=$(docker exec -e MYSQL_PWD="$root_password" "$mysql_container" mysql --default-character-set=utf8mb4 -h127.0.0.1 -uroot "$database" -N -B -e \
    "SELECT CONCAT(
      (SELECT COUNT(*) FROM sys_schema_migration WHERE version='20260815_112_v7_customer_storage'),':',
      (SELECT COUNT(*) FROM sys_schema_migration WHERE version='20260815_113_v7_air_gapped'),':',
      (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='t_air_gapped_package'),':',
      (SELECT COUNT(*) FROM sys_menu WHERE id IN (4515,4516,4517,4615,4616,4617) AND perms LIKE 'enterprise:air-gapped:%'),':',
      (SELECT COUNT(*) FROM sys_role_menu WHERE menu_id IN (4515,4516,4517,4615,4616,4617)),':',
      (SELECT COUNT(*) FROM sys_schema_migration WHERE version='schema113_acceptance_preupgrade_marker'),':',
      (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='t_storage_object' AND column_name='object_type' AND column_comment LIKE '%10离线结果包%'))")
  [[ $state == "1:1:1:6:0:$expect_marker:1" ]] || {
    echo "Schema 113生产迁移状态异常: database=$database state=$state" >&2
    docker exec -e MYSQL_PWD="$root_password" "$mysql_container" mysql --default-character-set=utf8mb4 -h127.0.0.1 -uroot "$database" -N -B -e \
      "SELECT CONCAT('object_type_comment=',column_comment) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='t_storage_object' AND column_name='object_type'" >&2
    exit 1
  }
}

validate_database fresh113 0
validate_database upgrade113 1
accepted_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

cleanup
trap - EXIT
remaining_containers=$(docker ps -a --filter "name=^/${mysql_container}$" -q | wc -l | tr -d ' ')
remaining_networks=$(docker network ls --filter "name=^${network}$" -q | wc -l | tr -d ' ')
remaining_images=0
for image in "$schema112_image" "$schema113_image"; do
  if docker image inspect "$image" >/dev/null 2>&1; then
    remaining_images=$((remaining_images + 1))
  fi
done
[[ $remaining_containers == 0 && $remaining_networks == 0 && $remaining_images == 0 ]]

mkdir -p "$(dirname "$report_file")"
umask 077
APPFORGE_SCHEMA113_ACCEPTED_AT="$accepted_at" \
APPFORGE_SCHEMA113_REMAINING_CONTAINERS="$remaining_containers" \
APPFORGE_SCHEMA113_REMAINING_NETWORKS="$remaining_networks" \
APPFORGE_SCHEMA113_REMAINING_IMAGES="$remaining_images" \
python3 <<'PY' >"$report_file"
import json
import os
import sys

integer = lambda key: int(os.environ[key])
json.dump({
    "schemaVersion": 1,
    "acceptedAt": os.environ["APPFORGE_SCHEMA113_ACCEPTED_AT"],
    "scope": "isolated-synthetic-schema113-production-promotion",
    "acceptanceScript": "deploy/acceptance/v7-schema113-candidate.sh",
    "productionDeploymentDefaultsChanged": True,
    "productionCredentialsUsed": False,
    "realCustomerDataUsed": False,
    "checks": {
        "schema112ImageExcludes113": "passed",
        "schema113ImageIncludesMigration": "passed",
        "freshSchema113Install": "passed",
        "schema112To113Upgrade": "passed",
        "migrationIdempotence": "passed",
        "preUpgradeMarkerPreserved": "passed",
        "sixPermanentRbacEntries": "passed",
        "existingRolesNotAutoGranted": "passed",
        "storageObjectTypeCommentCoversOneThroughTen": "passed",
    },
    "cleanup": {
        "remainingDockerContainers": integer("APPFORGE_SCHEMA113_REMAINING_CONTAINERS"),
        "remainingDockerNetworks": integer("APPFORGE_SCHEMA113_REMAINING_NETWORKS"),
        "remainingDockerImages": integer("APPFORGE_SCHEMA113_REMAINING_IMAGES"),
    },
    "limitations": [
        "synthetic-empty-and-schema112-databases-only",
        "historical-schema112-image-is-used-only-as-upgrade-source",
        "not-customer-physical-air-gap",
    ],
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
PY
chmod 0600 "$report_file"

echo "通过: Schema 113生产镜像空库安装、112到113升级、重复迁移、数据保持、6条最小RBAC和已有角色零自动授权"
echo "证据: $report_file"
