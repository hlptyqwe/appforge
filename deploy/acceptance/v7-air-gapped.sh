#!/usr/bin/env bash

set -euo pipefail

# 本脚本只允许在本地开发栈中创建和清理合成 AIR_GAPPED 数据。
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
api_url=${APPFORGE_AIR_GAPPED_API_URL:-http://127.0.0.1:8888}
network=${APPFORGE_DEV_NETWORK:-appforge-dev}
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
report_file=${APPFORGE_AIR_GAPPED_REPORT_FILE:-$repo_root/docs/enterprise/evidence/v7-air-gapped-20260815.json}
suffix="$(date +%s)-$$"
temporary=$(mktemp -d)
state_volume="appforge-air-gapped-state-$$"
wrong_state_volume="appforge-air-gapped-wrong-state-$$"
secret_volume="appforge-air-gapped-secret-$$"
media_volume="appforge-air-gapped-media-$$"
build_container="appforge-air-gapped-build-$$"
tenant_id=''
app_id=''
task_id=''
agent_id=''
role_id=''
air_gapped_menu_ids='4515,4516,4517,4615,4616,4617'
mysql=(docker exec appforge-mysql mysql -uappforge -pappforge_dev_password -D appforge -N -B)

cleanup() {
  docker rm -f "$build_container" "${build_container}-wrong" "${build_container}-tampered" "${build_container}-replay" >/dev/null 2>&1 || true
  docker volume rm "$state_volume" "$wrong_state_volume" "$secret_volume" "$media_volume" >/dev/null 2>&1 || true
  if [[ -n $tenant_id && $tenant_id =~ ^[1-9][0-9]*$ ]]; then
    docker run --rm --network "$network" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c \
      "mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null && mc rm --recursive --force local/appforge/tenants/$tenant_id >/dev/null 2>&1 || true" >/dev/null 2>&1 || true
    tables=$("${mysql[@]}" -e "SELECT DISTINCT TABLE_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA='appforge' AND COLUMN_NAME='tenant_id' AND TABLE_NAME<>'sys_tenant'" 2>/dev/null || true)
    for table in $tables; do
      [[ $table =~ ^[a-zA-Z0-9_]+$ ]] || continue
      "${mysql[@]}" -e "DELETE FROM \`$table\` WHERE tenant_id=$tenant_id" >/dev/null 2>&1 || true
    done
    "${mysql[@]}" -e "DELETE FROM sys_tenant WHERE id=$tenant_id" >/dev/null 2>&1 || true
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT

mkdir -p "$temporary/fixture" "$temporary/result"
chmod 0700 "$temporary" "$temporary/fixture" "$temporary/result"

grants_before_migration=$("${mysql[@]}" -e "SELECT COUNT(*) FROM sys_role_menu WHERE menu_id IN ($air_gapped_menu_ids)")
docker exec -i appforge-mysql mysql -uappforge -pappforge_dev_password -D appforge \
  <"$repo_root/deploy/mysql/migrations/113-v7-air-gapped.sql"
docker exec -i appforge-mysql mysql -uappforge -pappforge_dev_password -D appforge \
  <"$repo_root/deploy/mysql/migrations/113-v7-air-gapped.sql"
permanent_menu_count=$("${mysql[@]}" -e "SELECT COUNT(*) FROM sys_menu WHERE id IN ($air_gapped_menu_ids) AND perms IN ('enterprise:air-gapped:export','enterprise:air-gapped:import','enterprise:air-gapped:view')")
grants_after_migration=$("${mysql[@]}" -e "SELECT COUNT(*) FROM sys_role_menu WHERE menu_id IN ($air_gapped_menu_ids)")
[[ $permanent_menu_count == 6 && $grants_after_migration == "$grants_before_migration" ]] || {
  echo "Schema 113 AIR_GAPPED权限目录非幂等或迁移扩大了已有角色权限" >&2
  exit 1
}
docker compose -f "$repo_root/deploy/docker-compose.dev.yml" build core-rpc admin-api >/dev/null
docker compose -f "$repo_root/deploy/docker-compose.dev.yml" up -d --no-deps --force-recreate core-rpc >/dev/null
for _ in $(seq 1 60); do
  core_health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' appforge-core-rpc 2>/dev/null || true)
  [[ $core_health == healthy ]] && break
  sleep 1
done
[[ ${core_health:-} == healthy ]] || { docker logs appforge-core-rpc >&2; echo '最新 Core RPC 未健康启动' >&2; exit 1; }
docker compose -f "$repo_root/deploy/docker-compose.dev.yml" up -d --no-deps --force-recreate admin-api >/dev/null
for _ in $(seq 1 60); do
  api_health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' appforge-admin-api 2>/dev/null || true)
  [[ $api_health == healthy ]] && break
  sleep 1
done
[[ ${api_health:-} == healthy ]] || { docker logs appforge-admin-api >&2; echo '最新 Admin API 未健康启动' >&2; exit 1; }
docker build -f "$repo_root/deploy/docker/local-agent.Dockerfile" -t "$agent_image" "$repo_root" >/dev/null

docker run --rm --user 0 -v "$temporary/fixture:/out" --entrypoint sh "$agent_image" -lc '
set -eu
cat >/tmp/AndroidManifest.xml <<"EOF"
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.appforge.airgapped.acceptance" android:versionCode="1" android:versionName="1.0">
  <uses-sdk android:minSdkVersion="21" />
  <application android:allowBackup="false" android:label="AIR_GAPPED Acceptance" />
</manifest>
EOF
aapt package -f -M /tmp/AndroidManifest.xml -I /usr/share/android-framework-res/framework-res.apk -F /out/source.apk
keytool -genkeypair -noprompt -alias release -keyalg RSA -keysize 2048 -validity 30 \
  -dname "CN=AppForge AIR_GAPPED Acceptance,O=AppForge,C=CN" \
  -keystore /out/release.jks -storepass changeit -keypass changeit >/dev/null 2>&1
keytool -exportcert -alias release -keystore /out/release.jks -storepass changeit -file /out/release.der >/dev/null 2>&1
chmod 0644 /out/source.apk /out/release.jks /out/release.der
'

source_size=$(wc -c <"$temporary/fixture/source.apk" | tr -d ' ')
keystore_size=$(wc -c <"$temporary/fixture/release.jks" | tr -d ' ')
source_sha=$(shasum -a 256 "$temporary/fixture/source.apk" | awk '{print $1}')
keystore_sha=$(shasum -a 256 "$temporary/fixture/release.jks" | awk '{print $1}')
certificate_sha=$(shasum -a 256 "$temporary/fixture/release.der" | awk '{print $1}')
password_hash='$2y$10$bxPB8yeV4QLuCn5mNxzJB.cMVDtGXpRZiIF.r/u6c09RDBeUDGgaC'
username="airgap-$suffix"
pool_code="airgap-$suffix"

tenant_id=$("${mysql[@]}" -e "INSERT INTO sys_tenant (tenant_code,tenant_name,enabled,expire_time,remark,create_by,create_times,update_by,update_times) VALUES ('airgap-$suffix','AIR_GAPPED 临时验收租户',1,0,'自动清理，仅含合成数据','acceptance',UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000,'acceptance',UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000); SELECT LAST_INSERT_ID();" | tail -1)
[[ $tenant_id =~ ^[1-9][0-9]*$ ]] || { echo '创建临时租户失败' >&2; exit 1; }

source_key="tenants/$tenant_id/source-apk/air-gapped-$suffix/source.apk"
keystore_key="tenants/$tenant_id/keystore/air-gapped-$suffix/release.jks"
docker run --rm --network "$network" -v "$temporary/fixture:/fixture:ro" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
set -eu
mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
mc cp /fixture/source.apk local/appforge/$source_key >/dev/null
mc cp /fixture/release.jks local/appforge/$keystore_key >/dev/null
"

"${mysql[@]}" -e "
INSERT INTO sys_role (tenant_id,app_scope,name,code,enabled,remark,create_times,update_times) VALUES ($tenant_id,2,'AIR_GAPPED临时验收角色','airgap-acceptance-$tenant_id',1,'仅供合成验收并自动清理',UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000,UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000);
SET @role_id=LAST_INSERT_ID();
INSERT INTO sys_user (tenant_id,app_scope,user_type,is_owner,username,password,nickname,enabled,google_enabled,perms_ver,create_by,create_times,update_times) VALUES ($tenant_id,2,2,1,'$username','$password_hash','AIR_GAPPED Acceptance',1,2,1,0,UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000,UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000);
SET @user_id=LAST_INSERT_ID();
INSERT INTO sys_user_role (tenant_id,user_id,role_id) VALUES ($tenant_id,@user_id,@role_id);
INSERT INTO sys_role_menu (tenant_id,role_id,menu_id) VALUES ($tenant_id,@role_id,4610),($tenant_id,@role_id,3214),($tenant_id,@role_id,3215);
INSERT INTO t_tenant_subscription (tenant_id,plan_id,plan_version,status,source,current_period_start,current_period_end) VALUES ($tenant_id,2,1,1,3,CURRENT_TIMESTAMP(3),DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY));
SET @subscription_id=LAST_INSERT_ID();
INSERT INTO t_tenant_entitlement (tenant_id,source_type,source_id,plan_id,plan_version,builds_per_cycle,max_build_concurrency,storage_bytes,max_upload_bytes,team_seats,api_rate_limit,charge_failed_build,charge_cache_hit,charge_retry_build,valid_from,valid_until,status,revision) VALUES ($tenant_id,3,@subscription_id,2,1,-1,2,-1,-1,2,1000,0,1,1,CURRENT_TIMESTAMP(3),DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY),1,1);
INSERT INTO t_app_application (tenant_id,app_code,app_name,package_name,api_host,status,create_by) VALUES ($tenant_id,'airgap-$suffix','AIR_GAPPED Acceptance','com.appforge.airgapped.acceptance','https://api.acceptance.invalid',1,0);
SET @app_id=LAST_INSERT_ID();
INSERT INTO t_storage_object (tenant_id,app_id,object_type,object_key,original_name,content_type,size_bytes,sha256,status,create_by) VALUES
($tenant_id,@app_id,1,'$source_key','source.apk','application/vnd.android.package-archive',$source_size,'$source_sha',3,0),
($tenant_id,@app_id,2,'$keystore_key','release.jks','application/octet-stream',$keystore_size,'$keystore_sha',3,0);
SET @source_id=LAST_INSERT_ID(); SET @keystore_id=@source_id+1;
INSERT INTO t_app_version (tenant_id,app_id,version_code,version_name,source_apk_object_id,source_apk_sha256,status,create_by) VALUES ($tenant_id,@app_id,1,'1.0',@source_id,'$source_sha',2,0);
SET @version_id=LAST_INSERT_ID();
INSERT INTO t_promotion_channel (tenant_id,app_id,channel_code,channel_name,landing_url,status,create_by) VALUES ($tenant_id,@app_id,'airgap-$tenant_id','AIR_GAPPED Acceptance','https://acceptance.invalid',1,0);
SET @channel_id=LAST_INSERT_ID();
INSERT INTO t_app_signing_config (tenant_id,app_id,name,keystore_object_id,keystore_object_key,key_alias,certificate_sha256,secret_ref,status,create_by) VALUES ($tenant_id,@app_id,'AIR_GAPPED Acceptance',@keystore_id,'$keystore_key','release','$certificate_sha','local-file:///acceptance.json',1,0);
SET @signing_id=LAST_INSERT_ID();
INSERT INTO t_build_task (tenant_id,app_id,version_id,channel_id,signing_config_id,channel_code,version_code,version_name,source_apk_object_id,pool_code,status,builder_attempt,priority,queued_at,create_by) VALUES ($tenant_id,@app_id,@version_id,@channel_id,@signing_id,'airgap-$tenant_id',1,'1.0',@source_id,'$pool_code','PENDING',0,1000,CURRENT_TIMESTAMP(3),0);
SET @task_id=LAST_INSERT_ID();
INSERT INTO t_quota_reservation (tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,period_key,status,expires_at) VALUES ($tenant_id,'build.count',1,'build',@task_id,CONCAT('build:',@task_id),DATE_FORMAT(CURRENT_DATE,'%Y-%m'),1,DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY));
SELECT @app_id,@task_id,@role_id;
" >"$temporary/seed.txt"
read -r app_id task_id role_id <"$temporary/seed.txt"
[[ $app_id =~ ^[1-9][0-9]*$ && $task_id =~ ^[1-9][0-9]*$ && $role_id =~ ^[1-9][0-9]*$ ]] || { cat "$temporary/seed.txt" >&2; echo '创建临时业务数据失败' >&2; exit 1; }

login_response=$(curl -fsS -H 'Content-Type: application/json' -d "{\"username\":\"$username\",\"password\":\"AppForge@123\"}" "$api_url/agent/auth/login")
token=$(printf '%s' "$login_response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["token"])')
auth=(-H "Authorization: Bearer $token")

registration_response=$(curl -fsS "${auth[@]}" -H 'Content-Type: application/json' -d "{
  \"agentCode\":\"airgap-agent-$suffix\",\"agentName\":\"AIR_GAPPED Acceptance Agent\",\"poolCode\":\"$pool_code\",
  \"artifactMode\":3,\"allowedAppIds\":[$app_id],\"capabilities\":[{\"capabilityKey\":\"apk\",\"capabilityValue\":\"true\"},{\"capabilityKey\":\"max_concurrency\",\"capabilityValue\":\"1\"}],\"expiresSeconds\":900
}" "$api_url/agent/core/enterprise/local-agents")
agent_id=$(printf '%s' "$registration_response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])')
registration_token=$(printf '%s' "$registration_response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["registrationToken"])')
[[ $agent_id =~ ^[1-9][0-9]*$ ]] || { echo "$registration_response" >&2; exit 1; }

for volume in "$state_volume" "$wrong_state_volume" "$secret_volume" "$media_volume"; do
  docker volume create "$volume" >/dev/null
  docker run --rm --user 0 -v "$volume:/target" alpine:3.23 sh -c 'chown 65532:65532 /target; chmod 0700 /target'
done
printf '%s' "$registration_token" | docker run --rm -i --network "$network" -v "$state_volume:/var/lib/appforge-agent" "$agent_image" register \
  --control-url http://admin-api:8888 --gateway-url https://offline.invalid:9443 --token-stdin --state-dir /var/lib/appforge-agent >/dev/null
printf '%s' '{"keystorePassword":"changeit","keyPassword":"changeit"}' | docker run --rm -i \
  -v "$secret_volume:/etc/appforge/local-secrets" "$agent_image" secret-import --secret-root /etc/appforge/local-secrets --name acceptance.json --input-stdin >/dev/null

denied_export_code=$(curl -sS -o "$temporary/denied-export.txt" -w '%{http_code}' "${auth[@]}" -H 'Content-Type: application/json' \
  -d "{\"agentId\":$agent_id,\"taskId\":$task_id,\"expiresSeconds\":3600}" "$api_url/agent/core/enterprise/air-gapped/exports")
[[ $denied_export_code == 403 ]] || {
  cat "$temporary/denied-export.txt" >&2
  echo "未授权角色没有被AIR_GAPPED导出权限拒绝: HTTP $denied_export_code" >&2
  exit 1
}
"${mysql[@]}" -e "INSERT INTO sys_role_menu (tenant_id,role_id,menu_id) VALUES ($tenant_id,$role_id,4615),($tenant_id,$role_id,4616),($tenant_id,$role_id,4617)"
granted_air_permissions=$("${mysql[@]}" -e "SELECT COUNT(*) FROM sys_role_menu WHERE tenant_id=$tenant_id AND role_id=$role_id AND menu_id IN (4615,4616,4617)")
[[ $granted_air_permissions == 3 ]] || { echo 'AIR_GAPPED专用角色权限授予失败' >&2; exit 1; }

export_response=$(curl -fsS "${auth[@]}" -H 'Content-Type: application/json' -d "{\"agentId\":$agent_id,\"taskId\":$task_id,\"expiresSeconds\":3600}" \
  "$api_url/agent/core/enterprise/air-gapped/exports")
package_code=$(printf '%s' "$export_response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["package"]["packageCode"])')
download_url=$(printf '%s' "$export_response" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["downloadUrl"])')
curl -fsS "$download_url" -o "$temporary/result/task.zip"
chmod 0600 "$temporary/result/task.zip"

quota_state=$("${mysql[@]}" -e "SELECT CONCAT(t.status,':',t.builder_id,':',t.builder_attempt,':',q.status,':',p.status) FROM t_build_task t JOIN t_quota_reservation q ON q.resource_id=t.id JOIN t_air_gapped_package p ON p.task_id=t.id WHERE t.id=$task_id")
[[ $quota_state == "BUILDING:local-$agent_id:1:2:2" ]] || { echo "任务锁定、额度或包状态异常: $quota_state" >&2; exit 1; }

TASK_ZIP="$temporary/result/task.zip" SOURCE_APK="$temporary/fixture/source.apk" TAMPERED_ZIP="$temporary/result/tampered-task.zip" python3 - <<'PY'
import os
data=bytearray(open(os.environ['TASK_ZIP'],'rb').read())
source=open(os.environ['SOURCE_APK'],'rb').read()
offset=data.find(source)
assert offset >= 0
data[offset + len(source)//2] ^= 1
open(os.environ['TAMPERED_ZIP'],'wb').write(data)
PY
chmod 0600 "$temporary/result/tampered-task.zip"
docker run --rm --user 0 -v "$media_volume:/target" -v "$temporary/result:/source:ro" alpine:3.23 sh -c '
cp /source/task.zip /target/task.zip
cp /source/tampered-task.zip /target/tampered-task.zip
chown 65532:65532 /target/*.zip
chmod 0600 /target/*.zip
'
docker run --rm --user 0 -v "$state_volume:/source:ro" -v "$wrong_state_volume:/target" --entrypoint sh "$agent_image" -lc "
cp -a /source/. /target/
sed -i 's/\"agentId\": $agent_id/\"agentId\": $((agent_id + 1))/' /target/state.json
chown -R 65532:65532 /target
chmod 0700 /target
chmod 0600 /target/*
"

docker create --name "${build_container}-wrong" --network none -v "$wrong_state_volume:/var/lib/appforge-agent" -v "$secret_volume:/etc/appforge/local-secrets:ro" -v "$media_volume:/offline" \
  "$agent_image" air-gapped-build --task-package /offline/task.zip --result-package /offline/wrong-result.zip --state-dir /var/lib/appforge-agent --secret-root /etc/appforge/local-secrets >/dev/null
[[ $(docker inspect -f '{{.HostConfig.NetworkMode}}' "${build_container}-wrong") == none ]]
if docker start -a "${build_container}-wrong" >/dev/null 2>&1; then
  echo '错误Agent身份被离线任务包接受' >&2
  exit 1
fi

docker create --name "${build_container}-tampered" --network none -v "$state_volume:/var/lib/appforge-agent" -v "$secret_volume:/etc/appforge/local-secrets:ro" -v "$media_volume:/offline" \
  "$agent_image" air-gapped-build --task-package /offline/tampered-task.zip --result-package /offline/tampered-result.zip --state-dir /var/lib/appforge-agent --secret-root /etc/appforge/local-secrets >/dev/null
[[ $(docker inspect -f '{{.HostConfig.NetworkMode}}' "${build_container}-tampered") == none ]]
if docker start -a "${build_container}-tampered" >/dev/null 2>&1; then
  echo '被篡改的离线任务包被接受' >&2
  exit 1
fi

docker create --name "$build_container" --network none -v "$state_volume:/var/lib/appforge-agent" -v "$secret_volume:/etc/appforge/local-secrets:ro" -v "$media_volume:/offline" \
  "$agent_image" air-gapped-build --task-package /offline/task.zip --result-package /offline/result.zip --state-dir /var/lib/appforge-agent --secret-root /etc/appforge/local-secrets >/dev/null
[[ $(docker inspect -f '{{.HostConfig.NetworkMode}}' "$build_container") == none ]]
docker start -a "$build_container"

docker create --name "${build_container}-replay" --network none -v "$state_volume:/var/lib/appforge-agent" -v "$secret_volume:/etc/appforge/local-secrets:ro" -v "$media_volume:/offline" \
  "$agent_image" air-gapped-build --task-package /offline/task.zip --result-package /offline/replay-result.zip --state-dir /var/lib/appforge-agent --secret-root /etc/appforge/local-secrets >/dev/null
if docker start -a "${build_container}-replay" >/dev/null 2>&1; then
  echo 'Local Agent 本地防重放失效' >&2
  exit 1
fi

docker run --rm --user 0 -v "$media_volume:/source:ro" -v "$temporary/result:/target" alpine:3.23 sh -c 'cp /source/result.zip /target/result.zip; chmod 0600 /target/result.zip'
result_size=$(wc -c <"$temporary/result/result.zip" | tr -d ' ')
result_sha=$(shasum -a 256 "$temporary/result/result.zip" | awk '{print $1}')

upload_result() {
  local file=$1 output=$2 size ticket upload_url object_id
  size=$(wc -c <"$file" | tr -d ' ')
  ticket=$(curl -fsS "${auth[@]}" -H 'Content-Type: application/json' -d "{\"appId\":$app_id,\"objectType\":10,\"fileName\":\"$(basename "$file")\",\"sizeBytes\":$size,\"contentType\":\"application/zip\"}" "$api_url/agent/core/uploads/initiate")
  upload_url=$(printf '%s' "$ticket" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["uploadUrl"])')
  object_id=$(printf '%s' "$ticket" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["objectId"])')
  curl -fsS -X PUT -H 'Content-Type: application/zip' --data-binary @"$file" "$upload_url" >/dev/null
  curl -fsS "${auth[@]}" -X POST "$api_url/agent/core/uploads/$object_id/complete" >"$output"
  printf '%s' "$object_id"
}

RESULT_ZIP="$temporary/result/result.zip" TAMPERED_RESULT="$temporary/result/tampered-result.zip" python3 - <<'PY'
import os
data=bytearray(open(os.environ['RESULT_ZIP'],'rb').read())
needle=b'"signature"'
offset=data.find(needle)
assert offset >= 0
data[offset] ^= 1
open(os.environ['TAMPERED_RESULT'],'wb').write(data)
PY
chmod 0600 "$temporary/result/tampered-result.zip"
tampered_size=$(wc -c <"$temporary/result/tampered-result.zip" | tr -d ' ')
tampered_ticket=$(curl -fsS "${auth[@]}" -H 'Content-Type: application/json' -d "{\"appId\":$app_id,\"objectType\":10,\"fileName\":\"tampered-result.zip\",\"sizeBytes\":$tampered_size,\"contentType\":\"application/zip\"}" "$api_url/agent/core/uploads/initiate")
tampered_upload_url=$(printf '%s' "$tampered_ticket" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["uploadUrl"])')
tampered_object_id=$(printf '%s' "$tampered_ticket" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["objectId"])')
curl -fsS -X PUT -H 'Content-Type: application/zip' --data-binary @"$temporary/result/tampered-result.zip" "$tampered_upload_url" >/dev/null
tampered_complete_code=$(curl -sS -o "$temporary/tampered-complete.json" -w '%{http_code}' "${auth[@]}" -X POST "$api_url/agent/core/uploads/$tampered_object_id/complete")
if [[ $tampered_complete_code =~ ^2 ]]; then
  tampered_code=$(curl -sS -o "$temporary/tampered-import.json" -w '%{http_code}' "${auth[@]}" -H 'Content-Type: application/json' \
    -d "{\"packageCode\":\"$package_code\",\"resultObjectId\":$tampered_object_id}" "$api_url/agent/core/enterprise/air-gapped/imports")
  [[ $tampered_code =~ ^4 ]] || { cat "$temporary/tampered-import.json" >&2; echo "被篡改结果包未被拒绝: HTTP $tampered_code" >&2; exit 1; }
elif [[ ! $tampered_complete_code =~ ^4 ]]; then
  cat "$temporary/tampered-complete.json" >&2
  echo "被篡改结果包返回异常状态: HTTP $tampered_complete_code" >&2
  exit 1
fi

result_object_id=$(upload_result "$temporary/result/result.zip" "$temporary/result-complete.json")
import_response=$(curl -fsS "${auth[@]}" -H 'Content-Type: application/json' -d "{\"packageCode\":\"$package_code\",\"resultObjectId\":$result_object_id}" \
  "$api_url/agent/core/enterprise/air-gapped/imports")
printf '%s' "$import_response" | python3 -c 'import json,sys; p=json.load(sys.stdin); assert p["code"]==200 and p["data"]["status"]==3'

output_count=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_storage_object WHERE tenant_id=$tenant_id AND object_type IN (3,4) AND status=3")
idempotent_response=$(curl -fsS "${auth[@]}" -H 'Content-Type: application/json' -d "{\"packageCode\":\"$package_code\",\"resultObjectId\":$result_object_id}" \
  "$api_url/agent/core/enterprise/air-gapped/imports")
printf '%s' "$idempotent_response" | python3 -c 'import json,sys; p=json.load(sys.stdin); assert p["code"]==200 and p["data"]["status"]==3'
idempotent_output_count=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_storage_object WHERE tenant_id=$tenant_id AND object_type IN (3,4) AND status=3")
[[ $idempotent_output_count == "$output_count" ]] || { echo '幂等导入重复创建了APK或日志对象' >&2; exit 1; }

final_state=$("${mysql[@]}" -e "SELECT CONCAT(t.status,':',t.builder_attempt,':',p.status,':',s.status,':',COUNT(h.id)) FROM t_build_task t JOIN t_air_gapped_package p ON p.task_id=t.id JOIN t_build_slot_lease s ON s.task_id=t.id LEFT JOIN t_hybrid_artifact_reference h ON h.task_id=t.id AND h.builder_attempt=t.builder_attempt WHERE t.id=$task_id GROUP BY t.status,t.builder_attempt,p.status,s.status")
[[ $final_state == SUCCESS:1:3:2:4 ]] || { echo "导入事务最终状态异常: $final_state" >&2; exit 1; }

apk_object_id=$("${mysql[@]}" -e "SELECT apk_object_id FROM t_build_task WHERE id=$task_id")
apk_key=$("${mysql[@]}" -e "SELECT object_key FROM t_storage_object WHERE id=$apk_object_id AND tenant_id=$tenant_id")
docker run --rm --network "$network" -v "$temporary/result:/result" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
mc cp local/appforge/$apk_key /result/final.apk >/dev/null
"
docker run --rm --user 0 -v "$temporary/result:/result:ro" --entrypoint sh "$agent_image" -lc \
  "apksigner verify --verbose --print-certs /result/final.apk >/dev/null && aapt dump badging /result/final.apk | grep -q \"package: name='com.appforge.airgapped.acceptance'\""
final_apk_sha=$(shasum -a 256 "$temporary/result/final.apk" | awk '{print $1}')
task_package_sha=$(shasum -a 256 "$temporary/result/task.zip" | awk '{print $1}')

if docker logs "$build_container" 2>&1 | grep -Fq 'changeit'; then
  echo '离线构建日志泄漏了测试签名密码' >&2
  exit 1
fi
if "${mysql[@]}" -e "SELECT CONCAT_WS('|',package_code,COALESCE(export_sha256,''),COALESCE(result_sha256,'')) FROM t_air_gapped_package WHERE tenant_id=$tenant_id" | grep -Fq 'changeit'; then
  echo 'AIR_GAPPED数据库元数据泄漏了测试签名密码' >&2
  exit 1
fi

accepted_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
cleanup
trap - EXIT

remaining_tenant_count=$("${mysql[@]}" -e "SELECT COUNT(*) FROM sys_tenant WHERE id=$tenant_id")
remaining_menu_count=$("${mysql[@]}" -e "SELECT COUNT(*) FROM sys_menu WHERE id IN ($air_gapped_menu_ids) AND perms LIKE 'enterprise:air-gapped:%'")
remaining_air_gapped_grants=$("${mysql[@]}" -e "SELECT COUNT(*) FROM sys_role_menu WHERE tenant_id=$tenant_id AND menu_id IN ($air_gapped_menu_ids)")
remaining_object_count=$(docker run --rm --network "$network" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c \
  "mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null && mc find local/appforge/tenants/$tenant_id 2>/dev/null | wc -l")
remaining_volume_count=0
for volume in "$state_volume" "$wrong_state_volume" "$secret_volume" "$media_volume"; do
  if docker volume inspect "$volume" >/dev/null 2>&1; then
    remaining_volume_count=$((remaining_volume_count + 1))
  fi
done
remaining_container_count=0
for container in "$build_container" "${build_container}-wrong" "${build_container}-tampered" "${build_container}-replay"; do
  if docker container inspect "$container" >/dev/null 2>&1; then
    remaining_container_count=$((remaining_container_count + 1))
  fi
done
[[ $remaining_tenant_count == 0 && $remaining_menu_count == 6 && $remaining_air_gapped_grants == 0 && $remaining_object_count == 0 ]]
[[ $remaining_volume_count == 0 && $remaining_container_count == 0 && ! -e $temporary ]]

mkdir -p "$(dirname "$report_file")"
umask 077
APPFORGE_AIR_GAPPED_ACCEPTED_AT="$accepted_at" \
APPFORGE_AIR_GAPPED_PACKAGE_CODE="$package_code" \
APPFORGE_AIR_GAPPED_TASK_PACKAGE_SHA="$task_package_sha" \
APPFORGE_AIR_GAPPED_RESULT_PACKAGE_SHA="$result_sha" \
APPFORGE_AIR_GAPPED_RESULT_PACKAGE_SIZE="$result_size" \
APPFORGE_AIR_GAPPED_FINAL_APK_SHA="$final_apk_sha" \
APPFORGE_AIR_GAPPED_CERTIFICATE_SHA="$certificate_sha" \
APPFORGE_AIR_GAPPED_OUTPUT_COUNT="$output_count" \
APPFORGE_AIR_GAPPED_REMAINING_TENANTS="$remaining_tenant_count" \
APPFORGE_AIR_GAPPED_REMAINING_MENUS="$remaining_menu_count" \
APPFORGE_AIR_GAPPED_REMAINING_GRANTS="$remaining_air_gapped_grants" \
APPFORGE_AIR_GAPPED_REMAINING_OBJECTS="$remaining_object_count" \
APPFORGE_AIR_GAPPED_REMAINING_VOLUMES="$remaining_volume_count" \
APPFORGE_AIR_GAPPED_REMAINING_CONTAINERS="$remaining_container_count" \
python3 <<'PY' >"$report_file"
import json
import os
import sys

integer = lambda key: int(os.environ[key])
json.dump({
    "schemaVersion": 1,
    "acceptedAt": os.environ["APPFORGE_AIR_GAPPED_ACCEPTED_AT"],
    "scope": "synthetic-local-air-gapped",
    "acceptanceScript": "deploy/acceptance/v7-air-gapped.sh",
    "packageCode": os.environ["APPFORGE_AIR_GAPPED_PACKAGE_CODE"],
    "productionCredentialsUsed": False,
    "realCustomerDataUsed": False,
    "artifacts": {
        "taskPackageSha256": os.environ["APPFORGE_AIR_GAPPED_TASK_PACKAGE_SHA"],
        "resultPackageSha256": os.environ["APPFORGE_AIR_GAPPED_RESULT_PACKAGE_SHA"],
        "resultPackageSizeBytes": integer("APPFORGE_AIR_GAPPED_RESULT_PACKAGE_SIZE"),
        "finalApkSha256": os.environ["APPFORGE_AIR_GAPPED_FINAL_APK_SHA"],
        "certificateSha256": os.environ["APPFORGE_AIR_GAPPED_CERTIFICATE_SHA"],
        "importedOutputObjectCount": integer("APPFORGE_AIR_GAPPED_OUTPUT_COUNT"),
    },
    "checks": {
        "taskAndQuotaLocked": "passed",
        "controlPlaneSignatureVerified": "passed",
        "wrongAgentRejected": "passed",
        "taskTamperRejected": "passed",
        "networkModeNone": "passed",
        "realApkBuildAndSignature": "passed",
        "localReplayRejected": "passed",
        "agentResultSignatureVerified": "passed",
        "resultTamperRejected": "passed",
        "transactionalImport": "passed",
        "idempotentReimport": "passed",
        "secretLeakScan": "passed",
        "tenantPortalRoutes": "passed",
        "minimumRbacCatalogIdempotent": "passed",
        "defaultDenyBeforeExplicitGrant": "passed",
    },
    "cleanup": {
        "remainingTenantRows": integer("APPFORGE_AIR_GAPPED_REMAINING_TENANTS"),
        "permanentRbacMenuRows": integer("APPFORGE_AIR_GAPPED_REMAINING_MENUS"),
        "remainingTemporaryRoleGrants": integer("APPFORGE_AIR_GAPPED_REMAINING_GRANTS"),
        "remainingObjectPrefixEntries": integer("APPFORGE_AIR_GAPPED_REMAINING_OBJECTS"),
        "remainingDockerVolumes": integer("APPFORGE_AIR_GAPPED_REMAINING_VOLUMES"),
        "remainingDockerContainers": integer("APPFORGE_AIR_GAPPED_REMAINING_CONTAINERS"),
        "temporaryHostDirectoryRemoved": True,
    },
    "limitations": [
        "synthetic-apk-test-keystore-and-temporary-certificates-only",
        "local-docker-network-none-simulation-not-customer-physical-air-gap",
        "development-schema-113-not-production-schema-promotion",
    ],
}, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
PY
chmod 0600 "$report_file"

echo "通过: AIR_GAPPED租户端最小RBAC默认拒绝/显式授权、合成任务锁定、额度确认、CA签名导出、错误身份/输入篡改拒绝、--network none真实APK构建、本地防重放、Agent结果签名、结果篡改拒绝、事务导入、幂等与最终APK校验"
echo "证据: $report_file"
