#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d)
delivery="$temporary/production"
fixtures="$temporary/fixtures"
report_file=${APPFORGE_DR_REPORT_FILE:-}
schema_number=${APPFORGE_DR_SCHEMA_NUMBER:-112}
acceptance_script=${APPFORGE_DR_ACCEPTANCE_SCRIPT:-deploy/acceptance/v7-appforge-schema112-dr.sh}
case "$schema_number" in
  112)
    schema_target=20260815_112_v7_customer_storage
    appforge_version=1.1.0
    version_code=112
    version_name=1.1.2-dr
    schema_113_included=false
    ;;
  113)
    schema_target=20260815_113_v7_air_gapped
    appforge_version=1.2.0
    version_code=113
    version_name=1.2.0-dr
    schema_113_included=true
    ;;
  *)
    echo "验收失败: 仅支持 Schema 112 或 113，收到 $schema_number" >&2
    exit 1
    ;;
esac
project="appforge-v7-schema${schema_number}-dr-$$"
core_schema="$delivery/core-schema-${schema_number}.sql"
tenant_id=970001
app_id=970101
source_object_id=970201
keystore_object_id=970202
apk_object_id=970203
log_object_id=970204
offline_task_object_id=970205
offline_result_object_id=970206
version_id=970301
channel_id=970401
signing_config_id=970501
task_id=970601
audit_id=970701
air_gapped_package_id=970801
air_gapped_package_code="agp-v7-dr-schema-${schema_number}"
object_root="/data/appforge/tenants/$tenant_id/apps/$app_id"
storage_object_ids="$source_object_id,$keystore_object_id,$apk_object_id,$log_object_id"
expected_object_count=4
if [[ $schema_number == 113 ]]; then
  storage_object_ids+=",$offline_task_object_id,$offline_result_object_id"
  expected_object_count=6
fi
[[ $project =~ ^appforge-v7-schema(112|113)-dr-[0-9]+$ ]]

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

mkdir -p "$delivery/migrations" "$fixtures"
cp "$repo_root/deploy/production/backup.sh" "$delivery/backup.sh"
cp "$repo_root/deploy/production/restore.sh" "$delivery/restore.sh"
cp "$repo_root/services/system/system.sql" "$delivery/system.sql"
if [[ $schema_number == 112 ]]; then
  awk '
    /^-- APPFORGE_SCHEMA_113_BEGIN：/ { excluded=1; next }
    /^-- APPFORGE_SCHEMA_113_END$/ { excluded=0; next }
    !excluded { print }
  ' "$repo_root/services/core/core.sql" >"$core_schema"
  ! grep -q 't_air_gapped_package' "$core_schema"
else
  cp "$repo_root/services/core/core.sql" "$core_schema"
  grep -q 't_air_gapped_package' "$core_schema"
fi

for migration in "$repo_root"/deploy/mysql/migrations/*.sql; do
  migration_name=$(basename "$migration")
  migration_number=${migration_name%%-*}
  if ((10#$migration_number <= schema_number)); then
    cp "$migration" "$delivery/migrations/$migration_name"
  fi
done
[[ -s $delivery/migrations/112-v7-customer-storage.sql ]]
if [[ $schema_number == 112 ]]; then
  [[ ! -e $delivery/migrations/113-v7-air-gapped.sql ]]
else
  [[ -s $delivery/migrations/113-v7-air-gapped.sql ]]
fi

python3 - "$fixtures" "$schema_number" <<'PY'
import json
import pathlib
import sys
import zipfile

root = pathlib.Path(sys.argv[1])
schema_number = sys.argv[2]

def add_bytes(archive, name, value):
    info = zipfile.ZipInfo(name, (2026, 8, 15, 0, 0, 0))
    info.compress_type = zipfile.ZIP_STORED
    info.external_attr = 0o100600 << 16
    archive.writestr(info, value)

def make_apk(path, channel, built):
    with zipfile.ZipFile(path, "w") as archive:
        add_bytes(archive, "AndroidManifest.xml", b"synthetic-appforge-dr-manifest-v1")
        add_bytes(archive, "classes.dex", b"dex\n035\x00synthetic-appforge-dr")
        add_bytes(archive, "assets/channel.txt", channel.encode("ascii"))
        if built:
            add_bytes(archive, "META-INF/APPFORGE.SF", b"synthetic-test-signature")

make_apk(root / "synthetic-source.apk", "SOURCE", False)
make_apk(root / "synthetic-built.apk", "DR_SYNTHETIC", True)
(root / "synthetic-keystore.jks").write_bytes(b"APPFORGE-SYNTHETIC-TEST-KEYSTORE-V1\x00")
(root / "synthetic-build.log").write_text(
    f"synthetic AppForge Schema {schema_number} disaster recovery build log\n",
    encoding="utf-8",
)
with zipfile.ZipFile(root / "synthetic-offline-task.zip", "w") as archive:
    add_bytes(archive, "manifest.json", json.dumps({"schema": schema_number, "kind": "task"}, sort_keys=True).encode())
    add_bytes(archive, "inputs/source.apk", (root / "synthetic-source.apk").read_bytes())
with zipfile.ZipFile(root / "synthetic-offline-result.zip", "w") as archive:
    add_bytes(archive, "manifest.json", json.dumps({"schema": schema_number, "kind": "result"}, sort_keys=True).encode())
    add_bytes(archive, "outputs/app.apk", (root / "synthetic-built.apk").read_bytes())
    add_bytes(archive, "outputs/build.log", (root / "synthetic-build.log").read_bytes())
PY

file_sha256() {
  sha256sum "$1" | awk '{print $1}'
}

file_size() {
  wc -c <"$1" | tr -d ' '
}

source_sha256=$(file_sha256 "$fixtures/synthetic-source.apk")
source_size=$(file_size "$fixtures/synthetic-source.apk")
keystore_sha256=$(file_sha256 "$fixtures/synthetic-keystore.jks")
keystore_size=$(file_size "$fixtures/synthetic-keystore.jks")
apk_sha256=$(file_sha256 "$fixtures/synthetic-built.apk")
apk_size=$(file_size "$fixtures/synthetic-built.apk")
log_sha256=$(file_sha256 "$fixtures/synthetic-build.log")
log_size=$(file_size "$fixtures/synthetic-build.log")
offline_task_sha256=$(file_sha256 "$fixtures/synthetic-offline-task.zip")
offline_task_size=$(file_size "$fixtures/synthetic-offline-task.zip")
offline_result_sha256=$(file_sha256 "$fixtures/synthetic-offline-result.zip")
offline_result_size=$(file_size "$fixtures/synthetic-offline-result.zip")

if [[ ${APPFORGE_DR_FIXTURE_ONLY:-false} == true ]]; then
  unzip -t "$fixtures/synthetic-source.apk" >/dev/null
  unzip -t "$fixtures/synthetic-built.apk" >/dev/null
  unzip -t "$fixtures/synthetic-offline-task.zip" >/dev/null
  unzip -t "$fixtures/synthetic-offline-result.zip" >/dev/null
  grep -q 'CREATE TABLE sys_tenant' "$delivery/system.sql"
  grep -q 'CREATE TABLE t_app_application' "$core_schema"
  grep -q 'CREATE TABLE t_build_task' "$core_schema"
  [[ $source_size -gt 0 && $keystore_size -gt 0 && $apk_size -gt 0 && $log_size -gt 0 ]]
  [[ $offline_task_size -gt 0 && $offline_result_size -gt 0 ]]
  if [[ $schema_number == 112 ]]; then
    ! grep -q 't_air_gapped_package' "$core_schema"
  else
    grep -q 't_air_gapped_package' "$core_schema"
  fi
  echo "通过: Schema ${schema_number} 迁移边界与合成 APK/Keystore/构建产物/日志/离线包 fixture 静态校验"
  exit 0
fi

cat >"$delivery/docker-compose.yml" <<'YAML'
services:
  mysql:
    image: mysql:8.4
    command: ["--log-bin=mysql-bin", "--server-id=1", "--binlog-format=ROW", "--binlog-row-image=FULL"]
    environment:
      MYSQL_DATABASE: appforge
      MYSQL_USER: appforge
      MYSQL_PASSWORD: recovery_mysql_password
      MYSQL_ROOT_PASSWORD: recovery_root_password
    volumes: [mysql-data:/var/lib/mysql]
    healthcheck:
      test: ["CMD-SHELL", "MYSQL_PWD=$$MYSQL_ROOT_PASSWORD mysqladmin ping -uroot --silent"]
      interval: 2s
      timeout: 2s
      retries: 60
  etcd:
    image: quay.io/coreos/etcd:v3.6.12
    command: ["/usr/local/bin/etcd", "--name=default", "--data-dir=/etcd-data", "--listen-client-urls=http://0.0.0.0:2379", "--advertise-client-urls=http://etcd:2379"]
    volumes: [etcd-data:/etcd-data]
    healthcheck:
      test: ["CMD", "/usr/local/bin/etcdctl", "--endpoints=http://127.0.0.1:2379", "endpoint", "health"]
      interval: 2s
      timeout: 2s
      retries: 60
  minio:
    image: minio/minio:RELEASE.2025-04-22T22-12-26Z
    command: ["server", "/data", "--address", ":9000"]
    environment:
      MINIO_ROOT_USER: recovery-access
      MINIO_ROOT_PASSWORD: recovery-secret-password
    volumes: [object-data:/data]
  archive:
    image: mysql:8.4
    profiles: [tools]
    volumes: [object-data:/data, etcd-data:/etcd-data]
  api: &dummy
    image: mysql:8.4
    entrypoint: ["/bin/sh", "-c"]
    command: ["while true; do sleep 3600; done"]
  system-rpc: *dummy
  core-rpc: *dummy
  builder-rpc: *dummy
  builder-worker: *dummy
  webhook-worker: *dummy
  billing-worker: *dummy
  enterprise-worker: *dummy
  source-trigger-worker: *dummy
  admin-web: *dummy
  agent-web: *dummy
  minio-init: *dummy
volumes:
  mysql-data:
  etcd-data:
  object-data:
YAML

cat >"$delivery/.env" <<EOF
COMPOSE_PROJECT_NAME=$project
APPFORGE_MYSQL_DATABASE=appforge
APPFORGE_MYSQL_USER=appforge
APPFORGE_MYSQL_PASSWORD=recovery_mysql_password
APPFORGE_MYSQL_ROOT_PASSWORD=recovery_root_password
APPFORGE_VERSION=$appforge_version
APPFORGE_BACKUP_DIR=$temporary/backups
APPFORGE_RPO_MINUTES=15
APPFORGE_RTO_MINUTES=120
EOF
chmod 0600 "$delivery/.env"

compose=(docker compose --env-file "$delivery/.env" -f "$delivery/docker-compose.yml")

mysql_file() {
  "${compose[@]}" exec -T mysql sh -c \
    'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE"' <"$1"
}

mysql_query() {
  "${compose[@]}" exec -T mysql sh -c \
    'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot "$MYSQL_DATABASE" -N -B -e "$1"' sh "$1"
}

business_summary() {
  local air_gapped_summary=""
  if [[ $schema_number == 113 ]]; then
    air_gapped_summary="
    SELECT CONCAT_WS('|','AIRGAP',id,tenant_id,app_id,package_code,agent_id,task_id,builder_attempt,agent_certificate_serial,nonce_hash,export_object_id,export_sha256,export_size_bytes,result_object_id,result_sha256,result_size_bytes,status) FROM t_air_gapped_package WHERE id=$air_gapped_package_id;
    SELECT CONCAT('AIRGAPREFS|',COUNT(*))
      FROM t_air_gapped_package package_record
      JOIN t_build_task task ON task.id=package_record.task_id AND task.tenant_id=package_record.tenant_id AND task.builder_attempt=package_record.builder_attempt
      JOIN t_storage_object export_object ON export_object.id=package_record.export_object_id AND export_object.object_type=9 AND export_object.sha256=package_record.export_sha256
      JOIN t_storage_object result_object ON result_object.id=package_record.result_object_id AND result_object.object_type=10 AND result_object.sha256=package_record.result_sha256
      WHERE package_record.id=$air_gapped_package_id AND package_record.status=3;"
  fi
  mysql_query "
    SELECT CONCAT('SCHEMA|',version) FROM sys_schema_migration WHERE version='$schema_target';
    SELECT CONCAT('TABLES|',COUNT(*)) FROM information_schema.tables WHERE table_schema=DATABASE() AND (table_name LIKE 'sys\\_%' OR table_name LIKE 't\\_%');
    SELECT CONCAT_WS('|','TENANT',id,tenant_code,tenant_name,enabled,expire_time) FROM sys_tenant WHERE id=$tenant_id;
    SELECT CONCAT_WS('|','APP',id,tenant_id,app_code,app_name,package_name,status) FROM t_app_application WHERE id=$app_id;
    SELECT CONCAT_WS('|','OBJECT',id,tenant_id,app_id,object_type,object_key,size_bytes,sha256,status,storage_mode,owner_agent_id) FROM t_storage_object WHERE id IN ($storage_object_ids) ORDER BY id;
    SELECT CONCAT_WS('|','VERSION',id,tenant_id,app_id,version_code,version_name,source_apk_object_id,source_apk_sha256,status) FROM t_app_version WHERE id=$version_id;
    SELECT CONCAT_WS('|','CHANNEL',id,tenant_id,app_id,channel_code,channel_name,status) FROM t_promotion_channel WHERE id=$channel_id;
    SELECT CONCAT_WS('|','SIGNING',id,tenant_id,app_id,name,keystore_object_id,keystore_object_key,key_alias,certificate_sha256,status) FROM t_app_signing_config WHERE id=$signing_config_id;
    SELECT CONCAT_WS('|','TASK',id,tenant_id,app_id,version_id,channel_id,signing_config_id,channel_code,source_apk_object_id,status,builder_id,builder_attempt,apk_object_id,apk_sha256,apk_size,log_object_id) FROM t_build_task WHERE id=$task_id;
    SELECT CONCAT_WS('|','AUDIT',id,tenant_id,user_id,username,module,action,method,path,cost_ms) FROM sys_op_log WHERE id=$audit_id;
    $air_gapped_summary
    SELECT CONCAT('REFS|',COUNT(*))
      FROM t_build_task task
      JOIN sys_tenant tenant ON tenant.id=task.tenant_id
      JOIN t_app_application app ON app.id=task.app_id AND app.tenant_id=task.tenant_id
      JOIN t_app_version version ON version.id=task.version_id AND version.source_apk_object_id=task.source_apk_object_id
      JOIN t_promotion_channel channel ON channel.id=task.channel_id AND channel.app_id=task.app_id
      JOIN t_app_signing_config signing ON signing.id=task.signing_config_id AND signing.app_id=task.app_id
      JOIN t_storage_object source ON source.id=task.source_apk_object_id AND source.object_type=1
      JOIN t_storage_object keystore ON keystore.id=signing.keystore_object_id AND keystore.object_type=2
      JOIN t_storage_object apk ON apk.id=task.apk_object_id AND apk.object_type=3 AND apk.sha256=task.apk_sha256
      JOIN t_storage_object log_object ON log_object.id=task.log_object_id AND log_object.object_type=4
      JOIN sys_op_log audit ON audit.id=$audit_id AND audit.tenant_id=task.tenant_id
      WHERE task.id=$task_id;
  "
}

object_summary() {
  local offline_paths=""
  if [[ $schema_number == 113 ]]; then
    offline_paths="'$object_root/offline/$air_gapped_package_code/task.zip' '$object_root/offline/$air_gapped_package_code/result.zip'"
  fi
  "${compose[@]}" exec -T minio sh -c "sha256sum \
    '$object_root/source/synthetic-source.apk' \
    '$object_root/signing/synthetic-keystore.jks' \
    '$object_root/builds/$task_id/app.apk' \
    '$object_root/logs/$task_id/build.log' \
    $offline_paths"
}

"${compose[@]}" up -d --wait mysql etcd minio >"$temporary/start.log" 2>&1 || {
  tail -80 "$temporary/start.log" >&2
  exit 1
}
for _ in $(seq 1 30); do
  if "${compose[@]}" exec -T mysql sh -c \
    'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqladmin -h127.0.0.1 -uroot ping --silent' >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
"${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqladmin -h127.0.0.1 -uroot ping --silent' >/dev/null

mysql_file "$delivery/system.sql"
mysql_file "$core_schema"
while IFS= read -r migration; do
  mysql_file "$migration"
done < <(find "$delivery/migrations" -maxdepth 1 -type f -name '*.sql' | sort -V)

[[ $(mysql_query "SELECT COUNT(*) FROM sys_schema_migration WHERE version='$schema_target'") == 1 ]]
if [[ $schema_number == 112 ]]; then
  [[ $(mysql_query "SELECT COUNT(*) FROM sys_schema_migration WHERE version='20260815_113_v7_air_gapped'") == 0 ]]
  [[ $(mysql_query "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='t_air_gapped_package'") == 0 ]]
else
  [[ $(mysql_query "SELECT COUNT(*) FROM sys_schema_migration WHERE version='20260815_113_v7_air_gapped'") == 1 ]]
  [[ $(mysql_query "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='t_air_gapped_package'") == 1 ]]
fi

offline_storage_rows=""
if [[ $schema_number == 113 ]]; then
  offline_storage_rows=",
  ($offline_task_object_id,$tenant_id,$app_id,9,'tenants/$tenant_id/apps/$app_id/offline/$air_gapped_package_code/task.zip','task.zip','application/zip',$offline_task_size,'$offline_task_sha256',3,1,0,970001),
  ($offline_result_object_id,$tenant_id,$app_id,10,'tenants/$tenant_id/apps/$app_id/offline/$air_gapped_package_code/result.zip','result.zip','application/zip',$offline_result_size,'$offline_result_sha256',3,1,0,970001)"
fi
cat >"$delivery/seed.sql" <<SQL
INSERT INTO sys_tenant
  (id,tenant_code,tenant_name,enabled,expire_time,remark,create_by,create_times,update_by,update_times)
VALUES
  ($tenant_id,'v7-dr-synthetic','V7灾备合成租户',1,0,'仅用于隔离灾备验收','acceptance',1786723200000,'acceptance',1786723200000);
INSERT INTO t_app_application
  (id,tenant_id,app_code,app_name,package_name,description,status,create_by)
VALUES
  ($app_id,$tenant_id,'v7-dr-app','V7灾备合成应用','com.appforge.synthetic.dr','仅用于隔离灾备验收',1,970001);
INSERT INTO t_storage_object
  (id,tenant_id,app_id,object_type,object_key,original_name,content_type,size_bytes,sha256,status,storage_mode,owner_agent_id,create_by)
VALUES
  ($source_object_id,$tenant_id,$app_id,1,'tenants/$tenant_id/apps/$app_id/source/synthetic-source.apk','synthetic-source.apk','application/vnd.android.package-archive',$source_size,'$source_sha256',3,1,0,970001),
  ($keystore_object_id,$tenant_id,$app_id,2,'tenants/$tenant_id/apps/$app_id/signing/synthetic-keystore.jks','synthetic-keystore.jks','application/octet-stream',$keystore_size,'$keystore_sha256',3,1,0,970001),
  ($apk_object_id,$tenant_id,$app_id,3,'tenants/$tenant_id/apps/$app_id/builds/$task_id/app.apk','app.apk','application/vnd.android.package-archive',$apk_size,'$apk_sha256',3,1,0,970001),
  ($log_object_id,$tenant_id,$app_id,4,'tenants/$tenant_id/apps/$app_id/logs/$task_id/build.log','build.log','text/plain',$log_size,'$log_sha256',3,1,0,970001)$offline_storage_rows;
INSERT INTO t_app_version
  (id,tenant_id,app_id,version_code,version_name,source_apk_object_id,source_apk_url,source_apk_sha256,release_notes,build_config,status,create_by)
VALUES
  ($version_id,$tenant_id,$app_id,$version_code,'$version_name',$source_object_id,'storage-object://$source_object_id','$source_sha256','Schema $schema_number 合成灾备版本',JSON_OBJECT('scenario','v7-dr'),2,970001);
INSERT INTO t_promotion_channel
  (id,tenant_id,app_id,channel_code,channel_name,landing_url,download_url,status,create_by)
VALUES
  ($channel_id,$tenant_id,$app_id,'DR_SYNTHETIC','灾备合成渠道','https://invalid.example/dr','storage-object://$apk_object_id',1,970001);
INSERT INTO t_app_signing_config
  (id,tenant_id,app_id,name,keystore_object_id,keystore_object_key,key_alias,certificate_sha256,secret_ref,status,create_by)
VALUES
  ($signing_config_id,$tenant_id,$app_id,'灾备测试签名',$keystore_object_id,'tenants/$tenant_id/apps/$app_id/signing/synthetic-keystore.jks','appforge-dr','$keystore_sha256','local-file://synthetic-not-readable',1,970001);
INSERT INTO t_build_task
  (id,tenant_id,app_id,version_id,channel_id,signing_config_id,channel_code,version_code,version_name,source_apk_object_id,source_apk_url,build_config,pool_code,status,builder_id,builder_attempt,priority,apk_object_id,apk_url,apk_sha256,apk_size,log_object_id,log_url,queued_at,start_time,finish_time,create_by)
VALUES
  ($task_id,$tenant_id,$app_id,$version_id,$channel_id,$signing_config_id,'DR_SYNTHETIC',$version_code,'$version_name',$source_object_id,'storage-object://$source_object_id',JSON_OBJECT('scenario','v7-dr'),'isolated-dr','SUCCESS','synthetic-dr-builder',1,100,$apk_object_id,'storage-object://$apk_object_id','$apk_sha256',$apk_size,$log_object_id,'storage-object://$log_object_id','2026-08-15 00:00:00','2026-08-15 00:00:01','2026-08-15 00:00:02',970001);
INSERT INTO sys_op_log
  (id,tenant_id,user_id,username,module,action,method,path,req,resp,ip,cost_ms,create_times,update_times)
VALUES
  ($audit_id,$tenant_id,970001,'v7-dr-owner','build','synthetic-success','POST','/core/build-tasks','{\"taskId\":$task_id}','{\"status\":\"SUCCESS\"}','127.0.0.1',$schema_number,1786723202000,1786723202000);
SQL
if [[ $schema_number == 113 ]]; then
  cat >>"$delivery/seed.sql" <<SQL
INSERT INTO t_air_gapped_package
  (id,tenant_id,app_id,package_code,agent_id,task_id,builder_attempt,agent_certificate_serial,nonce_hash,export_object_id,export_sha256,export_size_bytes,result_object_id,result_sha256,result_size_bytes,status,issued_at,expires_at,imported_at,create_by)
VALUES
  ($air_gapped_package_id,$tenant_id,$app_id,'$air_gapped_package_code',970901,$task_id,1,'synthetic-agent-cert-970901','$source_sha256',$offline_task_object_id,'$offline_task_sha256',$offline_task_size,$offline_result_object_id,'$offline_result_sha256',$offline_result_size,3,'2026-08-15 00:00:00','2026-08-16 00:00:00','2026-08-15 00:00:02',970001);
SQL
fi
mysql_file "$delivery/seed.sql"

"${compose[@]}" exec -T minio sh -c "mkdir -p '$object_root/source' '$object_root/signing' '$object_root/builds/$task_id' '$object_root/logs/$task_id'"
"${compose[@]}" cp "$fixtures/synthetic-source.apk" "minio:$object_root/source/synthetic-source.apk"
"${compose[@]}" cp "$fixtures/synthetic-keystore.jks" "minio:$object_root/signing/synthetic-keystore.jks"
"${compose[@]}" cp "$fixtures/synthetic-built.apk" "minio:$object_root/builds/$task_id/app.apk"
"${compose[@]}" cp "$fixtures/synthetic-build.log" "minio:$object_root/logs/$task_id/build.log"
if [[ $schema_number == 113 ]]; then
  "${compose[@]}" exec -T minio mkdir -p "$object_root/offline/$air_gapped_package_code"
  "${compose[@]}" cp "$fixtures/synthetic-offline-task.zip" "minio:$object_root/offline/$air_gapped_package_code/task.zip"
  "${compose[@]}" cp "$fixtures/synthetic-offline-result.zip" "minio:$object_root/offline/$air_gapped_package_code/result.zip"
fi
"${compose[@]}" exec -T etcd etcdctl --endpoints=http://127.0.0.1:2379 \
  put "/appforge/v7-dr/tenants/$tenant_id" "{\"schema\":\"$schema_target\",\"taskId\":$task_id}" >/dev/null

before_business_summary=$(business_summary)
before_object_summary=$(object_summary)
[[ $(printf '%s\n' "$before_business_summary" | grep -c '^OBJECT|') == "$expected_object_count" ]]
[[ $(printf '%s\n' "$before_business_summary" | grep -c '^REFS|1$') == 1 ]]
if [[ $schema_number == 113 ]]; then
  [[ $(printf '%s\n' "$before_business_summary" | grep -c '^AIRGAP|') == 1 ]]
  [[ $(printf '%s\n' "$before_business_summary" | grep -c '^AIRGAPREFS|1$') == 1 ]]
fi
[[ $(printf '%s\n' "$before_object_summary" | grep -c .) == "$expected_object_count" ]]
business_digest=$(printf '%s' "$before_business_summary" | sha256sum | awk '{print $1}')
object_digest=$(printf '%s' "$before_object_summary" | sha256sum | awk '{print $1}')
recovery_marker_ms=$(now_millis)

APPFORGE_ENV_FILE="$delivery/.env" "$delivery/backup.sh" >"$temporary/backup.log" 2>&1 || {
  tail -120 "$temporary/backup.log" >&2
  exit 1
}
backup_completed_ms=$(now_millis)
backup_path=$(find "$temporary/backups" -mindepth 1 -maxdepth 1 -type d -print -quit)
[[ -n $backup_path && -f $backup_path/SHA256SUMS ]]
grep -q "\"schemaVersion\":\"$schema_target\"" "$backup_path/manifest.json"
rpo_seconds=$(((backup_completed_ms - recovery_marker_ms + 999) / 1000))
rpo_target_seconds=$((${APPFORGE_RPO_MINUTES:-15} * 60))
((rpo_seconds <= rpo_target_seconds)) || {
  echo "验收失败: 隔离恢复点间隔 ${rpo_seconds}s 超过目标 ${rpo_target_seconds}s" >&2
  exit 1
}

rto_started_ms=$(now_millis)
"${compose[@]}" exec -T mysql sh -c \
  'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql -h127.0.0.1 -uroot -e "DROP DATABASE IF EXISTS \`$MYSQL_DATABASE\`; CREATE DATABASE \`$MYSQL_DATABASE\` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci; GRANT ALL PRIVILEGES ON \`$MYSQL_DATABASE\`.* TO '\''$MYSQL_USER'\''@'\''%'\'';"'
"${compose[@]}" exec -T etcd etcdctl --endpoints=http://127.0.0.1:2379 del --prefix /appforge/v7-dr/ >/dev/null
"${compose[@]}" exec -T minio rm -rf "$object_root"
[[ $(mysql_query "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()") == 0 ]]
[[ -z $("${compose[@]}" exec -T etcd etcdctl --endpoints=http://127.0.0.1:2379 get --prefix /appforge/v7-dr/ --print-value-only) ]]
"${compose[@]}" exec -T minio test ! -e "$object_root"

APPFORGE_ENV_FILE="$delivery/.env" APPFORGE_RESTORE_CONFIRM="$backup_path" "$delivery/restore.sh" "$backup_path" >"$temporary/restore.log" 2>&1 || {
  tail -160 "$temporary/restore.log" >&2
  exit 1
}
"${compose[@]}" up -d --wait mysql etcd minio >"$temporary/restart.log" 2>&1 || {
  tail -80 "$temporary/restart.log" >&2
  exit 1
}

after_business_summary=$(business_summary)
after_object_summary=$(object_summary)
etcd_value=$("${compose[@]}" exec -T etcd etcdctl --endpoints=http://127.0.0.1:2379 \
  get "/appforge/v7-dr/tenants/$tenant_id" --print-value-only)
[[ $before_business_summary == "$after_business_summary" ]] || {
  echo "验收失败: Schema $schema_number 业务摘要恢复前后不一致" >&2
  diff -u <(printf '%s\n' "$before_business_summary") <(printf '%s\n' "$after_business_summary") >&2 || true
  exit 1
}
[[ $before_object_summary == "$after_object_summary" ]] || {
  echo "验收失败: 对象字节摘要恢复前后不一致" >&2
  diff -u <(printf '%s\n' "$before_object_summary") <(printf '%s\n' "$after_object_summary") >&2 || true
  exit 1
}
[[ $etcd_value == "{\"schema\":\"$schema_target\",\"taskId\":$task_id}" ]]
if [[ $schema_number == 112 ]]; then
  [[ $(mysql_query "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='t_air_gapped_package'") == 0 ]]
  [[ $(mysql_query "SELECT COUNT(*) FROM sys_schema_migration WHERE version='20260815_113_v7_air_gapped'") == 0 ]]
else
  [[ $(mysql_query "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='t_air_gapped_package'") == 1 ]]
  [[ $(mysql_query "SELECT COUNT(*) FROM t_air_gapped_package WHERE id=$air_gapped_package_id AND status=3") == 1 ]]
  [[ $(mysql_query "SELECT COUNT(*) FROM sys_schema_migration WHERE version='20260815_113_v7_air_gapped'") == 1 ]]
fi

rto_completed_ms=$(now_millis)
rto_seconds=$(((rto_completed_ms - rto_started_ms + 999) / 1000))
rto_target_seconds=$((${APPFORGE_RTO_MINUTES:-120} * 60))
((rto_seconds <= rto_target_seconds)) || {
  echo "验收失败: 隔离恢复耗时 ${rto_seconds}s 超过目标 ${rto_target_seconds}s" >&2
  exit 1
}

if [[ -n $report_file ]]; then
  mkdir -p "$(dirname "$report_file")"
  umask 077
  APPFORGE_DR_SCENARIO="isolated-synthetic-appforge-schema${schema_number}" \
    APPFORGE_DR_SCHEMA_NUMBER=$schema_number \
    APPFORGE_DR_SCHEMA_113_INCLUDED=$schema_113_included \
    APPFORGE_DR_ACCEPTANCE_SCRIPT=$acceptance_script \
    APPFORGE_DR_SCHEMA_TARGET=$schema_target \
    APPFORGE_DR_MARKER_MS=$recovery_marker_ms \
    APPFORGE_DR_BACKUP_MS=$backup_completed_ms \
    APPFORGE_DR_RESTORE_START_MS=$rto_started_ms \
    APPFORGE_DR_VERIFIED_MS=$rto_completed_ms \
    APPFORGE_DR_RPO_SECONDS=$rpo_seconds \
    APPFORGE_DR_RPO_TARGET_SECONDS=$rpo_target_seconds \
    APPFORGE_DR_RTO_SECONDS=$rto_seconds \
    APPFORGE_DR_RTO_TARGET_SECONDS=$rto_target_seconds \
    APPFORGE_DR_BUSINESS_DIGEST=$business_digest \
    APPFORGE_DR_OBJECT_DIGEST=$object_digest \
    APPFORGE_DR_TABLE_COUNT=$(mysql_query "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND (table_name LIKE 'sys\\_%' OR table_name LIKE 't\\_%')") \
    python3 -c '
import json,os,sys
integer=lambda key: int(os.environ[key])
boolean=lambda key: os.environ[key].lower() == "true"
json.dump({
  "schemaVersion": 3,
  "scenario": os.environ["APPFORGE_DR_SCENARIO"],
  "acceptanceScript": os.environ["APPFORGE_DR_ACCEPTANCE_SCRIPT"],
  "targetDatabaseSchema": os.environ["APPFORGE_DR_SCHEMA_TARGET"],
  "candidateSchema113Excluded": not boolean("APPFORGE_DR_SCHEMA_113_INCLUDED"),
  "productionSchema113Included": boolean("APPFORGE_DR_SCHEMA_113_INCLUDED"),
  "recoveryPointMarkerAt": integer("APPFORGE_DR_MARKER_MS"),
  "backupCompletedAt": integer("APPFORGE_DR_BACKUP_MS"),
  "restoreStartedAt": integer("APPFORGE_DR_RESTORE_START_MS"),
  "recoveryVerifiedAt": integer("APPFORGE_DR_VERIFIED_MS"),
  "rpoSeconds": integer("APPFORGE_DR_RPO_SECONDS"),
  "rpoTargetSeconds": integer("APPFORGE_DR_RPO_TARGET_SECONDS"),
  "rtoSeconds": integer("APPFORGE_DR_RTO_SECONDS"),
  "rtoTargetSeconds": integer("APPFORGE_DR_RTO_TARGET_SECONDS"),
  "appforgeTableCount": integer("APPFORGE_DR_TABLE_COUNT"),
  "businessSummarySha256": os.environ["APPFORGE_DR_BUSINESS_DIGEST"],
  "objectManifestSha256": os.environ["APPFORGE_DR_OBJECT_DIGEST"],
  "verifiedData": [
    "appforge-schema-" + os.environ["APPFORGE_DR_SCHEMA_NUMBER"],
    "synthetic-tenant-and-operation-audit",
    "synthetic-application-version-channel-signing-config",
    "synthetic-successful-build-task-reference-chain",
    ("six-storage-object-metadata-and-bytes-with-air-gapped-packages" if boolean("APPFORGE_DR_SCHEMA_113_INCLUDED") else "four-storage-object-metadata-and-bytes"),
    ("air-gapped-package-task-attempt-and-object-reference-chain" if boolean("APPFORGE_DR_SCHEMA_113_INCLUDED") else "schema-113-candidate-excluded"),
    "synthetic-etcd-config"
  ],
  "limitations": [
    "representative-synthetic-dataset-not-production-volume",
    "not-customer-target-environment",
    "not-pitr-or-cross-region"
  ]
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
' >"$report_file"
  chmod 0600 "$report_file"
fi
if [[ $schema_number == 112 ]]; then
  echo "通过: 隔离 AppForge Schema 112 合成业务链、etcd 与4个对象完成破坏恢复；Schema 113 被排除；RPO=${rpo_seconds}s/${rpo_target_seconds}s，RTO=${rto_seconds}s/${rto_target_seconds}s"
else
  echo "通过: 隔离 AppForge Schema 113 合成业务链、AIR_GAPPED 包、etcd 与6个对象完成破坏恢复；RPO=${rpo_seconds}s/${rpo_target_seconds}s，RTO=${rto_seconds}s/${rto_target_seconds}s"
fi
