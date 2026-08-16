#!/usr/bin/env bash

set -euo pipefail

# V2 负向 E2E：只允许在本机 appforge-dev 栈，或脚本创建的唯一隔离 Compose 栈中运行。
# 共享开发栈默认拒绝执行；隔离栈由 v2-branding-isolated-e2e.sh 创建并在退出时销毁。
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
api_url=${APPFORGE_V2_API_URL:-http://127.0.0.1:8888}
mysql_container=${APPFORGE_V2_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V2_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V2_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V2_MYSQL_PASSWORD:-appforge_dev_password}
network=${APPFORGE_DEV_NETWORK:-appforge-dev}
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
compose_project_expected=${APPFORGE_V2_COMPOSE_PROJECT:-appforge-dev}
isolated_project=${APPFORGE_V2_ISOLATED_PROJECT:-}
core_container=${APPFORGE_V2_CORE_CONTAINER:-appforge-core-rpc}
builder_rpc_container=${APPFORGE_V2_BUILDER_RPC_CONTAINER:-appforge-builder-rpc}
api_container=${APPFORGE_V2_API_CONTAINER:-appforge-admin-api}
minio_container=${APPFORGE_V2_MINIO_CONTAINER:-appforge-minio}
expected_upload_origin=${APPFORGE_V2_EXPECT_UPLOAD_ORIGIN:-}
portal_prefix=${APPFORGE_V2_PORTAL_PREFIX:-/agent}
admin_username=${APPFORGE_V2_ADMIN_USERNAME:-agent}
admin_password=${APPFORGE_V2_ADMIN_PASSWORD:-AppForge@123}
evidence=${APPFORGE_V2_NEGATIVE_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v2-branding-negative-e2e-20260815.json}
suffix="$(date +%s)-$$"
package_name="com.appforge.v2.negative.n$(printf '%s' "$suffix" | tr -d '-')"
temporary=$(mktemp -d)
app_id=''
tenant_id=''
baseline_storage_usage=''
object_keys_file="$temporary/object-keys"
mysql=(docker exec -e MYSQL_PWD="$mysql_password" "$mysql_container" mysql -u"$mysql_user" -D "$mysql_database" --default-character-set=utf8mb4 -N -B)

cleanup_fixture() {
  if [[ -z $app_id || ! $app_id =~ ^[1-9][0-9]*$ ]]; then
    return 0
  fi
  "${mysql[@]}" -e "SELECT object_key FROM t_storage_object WHERE app_id=$app_id AND object_key REGEXP '^tenants/[0-9]+/(source-apk|brand-logo|brand-splash)/'" \
    >>"$object_keys_file" 2>/dev/null || true
  if [[ -s $object_keys_file ]]; then
    sort -u "$object_keys_file" | while IFS= read -r object_key; do
      [[ $object_key =~ ^tenants/[0-9]+/(source-apk|brand-logo|brand-splash)/[A-Za-z0-9._/-]+$ ]] || continue
      docker run --rm --network "$network" -e OBJECT_KEY="$object_key" --entrypoint /bin/sh \
        minio/mc:RELEASE.2025-04-16T18-13-26Z -c '
          mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
          mc rm --force "local/appforge/$OBJECT_KEY" >/dev/null
        ' >/dev/null 2>&1 || true
    done
  fi
  "${mysql[@]}" -e "
INSERT INTO t_usage_ledger
  (tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,occurred_at,period_key,metadata)
SELECT o.tenant_id,
       CASE WHEN o.object_type IN (3,8) THEN 'storage.artifact_bytes'
            WHEN o.object_type=4 THEN 'storage.log_bytes'
            ELSE 'storage.source_bytes' END,
       -o.size_bytes,'acceptance.cleanup',o.id,
       CONCAT('v2-negative-cleanup:',o.id),CURRENT_TIMESTAMP(3),DATE_FORMAT(CURRENT_TIMESTAMP(3),'%Y-%m'),
       JSON_OBJECT('reason','V2 synthetic negative E2E cleanup','appId',o.app_id)
FROM t_storage_object o
WHERE o.app_id=$app_id AND o.size_bytes>0 AND o.status IN (2,3)
  AND EXISTS (
    SELECT 1 FROM t_usage_ledger positive
    WHERE positive.tenant_id=o.tenant_id AND positive.resource_type='storage'
      AND positive.resource_id=o.id AND positive.quantity=o.size_bytes
  )
  AND NOT EXISTS (
    SELECT 1 FROM t_usage_ledger cleanup
    WHERE cleanup.tenant_id=o.tenant_id
      AND cleanup.metric=CASE WHEN o.object_type IN (3,8) THEN 'storage.artifact_bytes'
                              WHEN o.object_type=4 THEN 'storage.log_bytes'
                              ELSE 'storage.source_bytes' END
      AND cleanup.idempotency_key=CONCAT('v2-negative-cleanup:',o.id)
  );
DELETE FROM t_quota_reservation
WHERE resource_type='storage' AND resource_id IN (
  SELECT id FROM t_storage_object WHERE app_id=$app_id
);
" >/dev/null 2>&1 || true
  "${mysql[@]}" -e "
DELETE FROM t_build_task WHERE app_id=$app_id;
DELETE FROM t_branding_preflight WHERE app_id=$app_id;
DELETE FROM t_app_branding_profile WHERE app_id=$app_id;
DELETE FROM t_app_version WHERE app_id=$app_id;
DELETE FROM t_storage_object WHERE app_id=$app_id;
DELETE FROM t_app_application WHERE id=$app_id AND app_code='v2-negative-$suffix';
" >/dev/null 2>&1 || true
}

cleanup() {
  cleanup_fixture || true
  rm -rf "$temporary"
}
trap cleanup EXIT
trap 'echo "V2 负向 E2E 在第 ${LINENO} 行失败；已尝试精确清理本次合成数据" >&2' ERR

if [[ ${APPFORGE_V2_FIXTURE_ONLY:-false} == true ]]; then
  for command in docker grep mktemp tr; do
    command -v "$command" >/dev/null || { echo "缺少命令: $command" >&2; exit 1; }
  done
  docker image inspect "$agent_image" >/dev/null
  mkdir -p "$temporary/fixture"
  chmod 0700 "$temporary" "$temporary/fixture"
  docker run --rm --user 0 -e PACKAGE_NAME="$package_name" -v "$temporary/fixture:/out" --entrypoint sh "$agent_image" -lc '
set -eu
cat >/tmp/AndroidManifest.xml <<EOF
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="$PACKAGE_NAME" android:versionCode="1" android:versionName="1.0">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="35" />
  <application android:label="V2 Negative"><activity android:name=".MainActivity" android:exported="true"><intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter></activity></application>
</manifest>
EOF
aapt package -f -M /tmp/AndroidManifest.xml -I /usr/share/android-framework-res/framework-res.apk -F /out/incompatible.apk
aapt dump badging /out/incompatible.apk | grep -F "$PACKAGE_NAME" >/dev/null
apktool d -f /out/incompatible.apk -o /tmp/v2-negative-decoded >/dev/null
grep -F "android:label=\"V2 Negative\"" /tmp/v2-negative-decoded/AndroidManifest.xml >/dev/null
if [ -d /tmp/v2-negative-decoded/res ] && find /tmp/v2-negative-decoded/res -type f -path "*/mipmap*/ic_launcher.*" | grep -q .; then
  echo "负向 fixture 意外包含 mipmap/ic_launcher" >&2
  exit 1
fi
if [ -d /tmp/v2-negative-decoded/res ] && find /tmp/v2-negative-decoded/res -type f -path "*/drawable*/splash_logo.*" | grep -q .; then
  echo "负向 fixture 意外包含 drawable/splash_logo" >&2
  exit 1
fi
apktool b /tmp/v2-negative-decoded -o /tmp/v2-negative-rebuilt.apk >/dev/null
test -s /tmp/v2-negative-rebuilt.apk
'
  echo 'V2 负向 fixture 通过：APK 可解析/解码/重建、应用名存在，Launcher 与启动图目标均缺失；未访问 API、数据库或对象存储。'
  exit 0
fi

[[ ${APPFORGE_V2_ALLOW_WRITE_E2E:-false} == true ]] || {
  echo '拒绝执行：未设置 APPFORGE_V2_ALLOW_WRITE_E2E=true；本脚本会写入并清理本地开发数据' >&2
  exit 2
}
case "$api_url" in
  http://127.0.0.1:*|http://localhost:*) ;;
  *) echo "拒绝非本机 API: $api_url" >&2; exit 2 ;;
esac
[[ $portal_prefix == /admin || $portal_prefix == /agent ]] || {
  echo '拒绝未知 V2 门户前缀' >&2
  exit 2
}
if [[ $portal_prefix == /agent ]]; then
  login_path=/agent/auth/login
else
  login_path=/admin/system/auth/login
fi
if [[ -n $isolated_project ]]; then
  [[ $isolated_project =~ ^appforge-v2-isolated-[0-9]+-[0-9]+$ ]] || {
    echo '拒绝非法 V2 隔离 Compose 项目标识' >&2
    exit 2
  }
  [[ $compose_project_expected == "$isolated_project" ]] || {
    echo '拒绝隔离项目标识与预期 Compose 项目不一致' >&2
    exit 2
  }
  [[ $mysql_container == "$isolated_project-mysql-1" && $network == "${isolated_project}_appforge" ]] || {
    echo '拒绝隔离项目之外的 MySQL 容器或网络' >&2
    exit 2
  }
else
  [[ $mysql_container == appforge-mysql && $network == appforge-dev && $compose_project_expected == appforge-dev ]] || {
    echo '拒绝非标准 appforge-dev MySQL 容器、网络或 Compose 项目' >&2
    exit 2
  }
fi

for command in awk basename curl docker python3 seq shasum sort tr wc; do
  command -v "$command" >/dev/null || { echo "缺少命令: $command" >&2; exit 1; }
done
compose_project=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' "$mysql_container" 2>/dev/null || true)
[[ $compose_project == "$compose_project_expected" ]] || { echo '拒绝非预期 Compose 项目数据库' >&2; exit 2; }
for container in "$mysql_container" "$core_container" "$builder_rpc_container" "$api_container" "$minio_container"; do
  state=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)
  [[ $state == healthy || $state == running ]] || { echo "V2 E2E 依赖容器不健康: $container ($state)" >&2; exit 1; }
done
worker_count=$(docker ps --filter "label=com.docker.compose.project=$compose_project_expected" \
  --format '{{.Label "com.docker.compose.service"}}' | awk '/^builder-worker-/{count++} END{print count+0}')
[[ $worker_count -ge 1 ]] || {
  echo '没有运行中的真实 Builder Worker' >&2
  exit 1
}
eligible_builder_count=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_builder_node
WHERE status=1 AND drain_status=1 AND last_heartbeat_at>=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 90 SECOND)
  AND max_concurrency>running_count AND disk_capacity>0
  AND disk_free>=536870912 AND disk_free*100>=disk_capacity*2
  AND build_protocol_version>=1 AND toolchain_version<>''
  AND JSON_VALID(capability_json) AND JSON_EXTRACT(capability_json,'$.branding')=TRUE")
[[ $eligible_builder_count -ge 1 ]] || {
  echo '没有满足在线、非排空、心跳、并发、磁盘和 branding capability 门禁的 Builder；请先恢复 V4 节点' >&2
  exit 1
}
docker image inspect "$agent_image" >/dev/null
mkdir -p "$temporary/fixture" "$temporary/responses"
chmod 0700 "$temporary" "$temporary/fixture" "$temporary/responses"

docker run --rm --user 0 -e PACKAGE_NAME="$package_name" -v "$temporary/fixture:/out" --entrypoint sh "$agent_image" -lc '
set -eu
cat >/tmp/AndroidManifest.xml <<EOF
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="$PACKAGE_NAME" android:versionCode="1" android:versionName="1.0">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="35" />
  <application android:label="V2 Negative"><activity android:name=".MainActivity" android:exported="true"><intent-filter><action android:name="android.intent.action.MAIN"/><category android:name="android.intent.category.LAUNCHER"/></intent-filter></activity></application>
</manifest>
EOF
aapt package -f -M /tmp/AndroidManifest.xml -I /usr/share/android-framework-res/framework-res.apk -F /out/incompatible.apk
chmod 0644 /out/incompatible.apk
'

FIXTURE_DIR="$temporary/fixture" python3 - <<'PY'
import binascii
import os
import pathlib
import struct
import zlib

root = pathlib.Path(os.environ['FIXTURE_DIR'])

def chunk(kind, payload):
    return struct.pack('>I', len(payload)) + kind + payload + struct.pack('>I', binascii.crc32(kind + payload) & 0xffffffff)

def solid_png(path, width, height, rgba):
    row = b'\x00' + bytes(rgba) * width
    raw = row * height
    png = b'\x89PNG\r\n\x1a\n'
    png += chunk(b'IHDR', struct.pack('>IIBBBBB', width, height, 8, 6, 0, 0, 0))
    png += chunk(b'IDAT', zlib.compress(raw, 9))
    png += chunk(b'IEND', b'')
    path.write_bytes(png)

solid_png(root / 'logo.png', 512, 512, (35, 120, 245, 255))
solid_png(root / 'splash.png', 720, 1280, (245, 248, 255, 255))
PY

login_response=$(curl -fsS -H 'Content-Type: application/json' \
  -d "{\"username\":\"$admin_username\",\"password\":\"$admin_password\"}" \
  "$api_url$login_path")
token=$(printf '%s' "$login_response" | python3 -c 'import json,sys; p=json.load(sys.stdin); assert p["code"]==200; print(p["data"]["token"])')
auth=(-H "Authorization: Bearer $token")

api_post() {
  local path=$1 body=$2 output=$3
  curl -fsS "${auth[@]}" -H 'Content-Type: application/json' -d "$body" "$api_url$path" >"$output"
  RESPONSE_FILE="$output" python3 - <<'PY'
import json, os
p=json.load(open(os.environ['RESPONSE_FILE'], encoding='utf-8'))
assert p['code'] == 200, p
PY
}

json_data_id() {
  RESPONSE_FILE=$1 python3 -c 'import json,os; print(json.load(open(os.environ["RESPONSE_FILE"], encoding="utf-8"))["data"]["id"])'
}

app_response="$temporary/responses/application.json"
api_post "$portal_prefix/core/applications" \
  "{\"appCode\":\"v2-negative-$suffix\",\"appName\":\"V2 负向合成验收\",\"packageName\":\"$package_name\",\"apiHost\":\"https://v2-negative.invalid\",\"description\":\"自动清理，不包含客户数据\"}" \
  "$app_response"
app_id=$(json_data_id "$app_response")
[[ $app_id =~ ^[1-9][0-9]*$ ]]
tenant_id=$(RESPONSE_FILE="$app_response" python3 -c 'import json,os; print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["tenantId"])')
[[ $tenant_id =~ ^[1-9][0-9]*$ ]]
baseline_storage_usage=$("${mysql[@]}" -e "SELECT COALESCE(SUM(quantity),0) FROM t_usage_ledger WHERE tenant_id=$tenant_id AND metric IN ('storage.source_bytes','storage.artifact_bytes','storage.log_bytes')")
[[ $baseline_storage_usage =~ ^-?[0-9]+$ ]]

upload_file() {
  local path=$1 object_type=$2 content_type=$3 label=$4
  local size ticket_file complete_file object_id upload_url
  size=$(wc -c <"$path" | tr -d ' ')
  ticket_file="$temporary/responses/upload-$label.json"
  complete_file="$temporary/responses/upload-$label-complete.json"
  api_post "$portal_prefix/core/uploads/initiate" \
    "{\"appId\":$app_id,\"objectType\":$object_type,\"fileName\":\"$(basename "$path")\",\"sizeBytes\":$size,\"contentType\":\"$content_type\"}" \
    "$ticket_file"
  object_id=$(RESPONSE_FILE="$ticket_file" python3 -c 'import json,os; print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["objectId"])')
  upload_url=$(RESPONSE_FILE="$ticket_file" python3 -c 'import json,os; print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["uploadUrl"])')
  if [[ -n $expected_upload_origin ]]; then
    case "$upload_url" in
      "$expected_upload_origin"/*) ;;
      *) echo "拒绝非预期隔离 MinIO 上传 URL: $upload_url" >&2; return 1 ;;
    esac
  else
    case "$upload_url" in
      http://127.0.0.1:*|http://localhost:*) ;;
      *) echo "拒绝非本机预签名上传 URL: $upload_url" >&2; return 1 ;;
    esac
  fi
  curl -fsS -X PUT -H "Content-Type: $content_type" --data-binary "@$path" "$upload_url" >/dev/null
  api_post "$portal_prefix/core/uploads/$object_id/complete" '{}' "$complete_file"
  printf '%s' "$object_id"
}

source_object_id=$(upload_file "$temporary/fixture/incompatible.apk" 1 application/vnd.android.package-archive source)
logo_object_id=$(upload_file "$temporary/fixture/logo.png" 5 image/png logo)
splash_object_id=$(upload_file "$temporary/fixture/splash.png" 6 image/png splash)
for object_id in "$source_object_id" "$logo_object_id" "$splash_object_id"; do [[ $object_id =~ ^[1-9][0-9]*$ ]]; done

version_response="$temporary/responses/version.json"
api_post "$portal_prefix/core/versions" \
  "{\"appId\":$app_id,\"versionCode\":1,\"versionName\":\"1.0-negative\",\"sourceApkObjectId\":$source_object_id,\"releaseNotes\":\"V2 incompatible synthetic APK\"}" \
  "$version_response"
version_id=$(json_data_id "$version_response")

profile_response="$temporary/responses/profile.json"
api_post "$portal_prefix/core/branding-profiles" \
  "{\"appId\":$app_id,\"profileName\":\"v2-negative-$suffix\",\"appName\":\"V2 负向验收\",\"logoObjectId\":$logo_object_id,\"splashObjectId\":$splash_object_id,\"apiHost\":\"https://v2-negative.invalid\",\"rewriteMode\":1,\"launcherIconTarget\":\"mipmap/ic_launcher\",\"splashResourceTarget\":\"drawable/splash_logo\",\"runtimeConfigJson\":\"{}\"}" \
  "$profile_response"
profile_id=$(json_data_id "$profile_response")

preflight_create="$temporary/responses/preflight-create.json"
api_post "$portal_prefix/core/branding-profiles/$profile_id/preflight" "{\"versionId\":$version_id}" "$preflight_create"
preflight_id=$(json_data_id "$preflight_create")

preflight_result="$temporary/responses/preflight-result.json"
preflight_status=''
for _ in $(seq 1 120); do
  curl -fsS "${auth[@]}" "$api_url$portal_prefix/core/branding-preflights/$preflight_id" >"$preflight_result"
  preflight_status=$(RESPONSE_FILE="$preflight_result" python3 -c 'import json,os; p=json.load(open(os.environ["RESPONSE_FILE"])); assert p["code"]==200; print(p["data"]["status"])')
  [[ $preflight_status != 1 ]] && break
  sleep 1
done
[[ $preflight_status == 3 ]] || {
  echo "真实 Builder 未返回 INCOMPATIBLE，当前状态: ${preflight_status:-unknown}" >&2
  exit 1
}

REPORT_FILE="$preflight_result" python3 - <<'PY'
import json, os
p=json.load(open(os.environ['REPORT_FILE'], encoding='utf-8'))
assert p['code'] == 200 and p['data']['status'] == 3
report=json.loads(p['data']['reportJson'])
assert report['schemaVersion'] == 1 and report['compatible'] is False
checks={item['name']: item for item in report['checks']}
assert checks['apktool_decode']['passed'] is True
assert checks['application_label']['passed'] is True
assert checks['launcher_icon']['passed'] is False
assert 'mipmap/ic_launcher' in checks['launcher_icon']['message']
assert checks['splash_resource']['passed'] is False
assert 'drawable/splash_logo' in checks['splash_resource']['message']
assert checks['apktool_rebuild']['passed'] is True
PY

build_list="$temporary/responses/build-list.json"
curl -fsS "${auth[@]}" "$api_url$portal_prefix/core/build-tasks?appId=$app_id&limit=100" >"$build_list"
BUILD_LIST="$build_list" python3 - <<'PY'
import json, os
p=json.load(open(os.environ['BUILD_LIST'], encoding='utf-8'))
assert p['code'] == 200 and p.get('total', 0) == 0 and p['data'] == [], p
PY
db_build_count=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_build_task WHERE app_id=$app_id")
[[ $db_build_count == 0 ]]
read -r builder_id builder_attempt toolchain source_sha <<<"$("${mysql[@]}" -e "SELECT builder_id,builder_attempt,toolchain_version,source_apk_sha256 FROM t_branding_preflight WHERE id=$preflight_id AND app_id=$app_id AND status=3")"
[[ -n $builder_id && $builder_attempt -ge 1 && -n $toolchain && $source_sha =~ ^[0-9a-f]{64}$ ]]
fixture_sha=$(shasum -a 256 "$temporary/fixture/incompatible.apk" | awk '{print $1}')
[[ $fixture_sha == "$source_sha" ]]
report_json=$(RESPONSE_FILE="$preflight_result" python3 -c 'import json,os; print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["reportJson"])')

cleanup_fixture
remaining=$("${mysql[@]}" -e "SELECT
  (SELECT COUNT(*) FROM t_app_application WHERE id=$app_id) +
  (SELECT COUNT(*) FROM t_app_version WHERE app_id=$app_id) +
  (SELECT COUNT(*) FROM t_app_branding_profile WHERE app_id=$app_id) +
  (SELECT COUNT(*) FROM t_branding_preflight WHERE app_id=$app_id) +
  (SELECT COUNT(*) FROM t_build_task WHERE app_id=$app_id) +
  (SELECT COUNT(*) FROM t_storage_object WHERE app_id=$app_id)")
[[ $remaining == 0 ]] || { echo "V2 临时数据未清理干净: $remaining" >&2; exit 1; }
final_storage_usage=$("${mysql[@]}" -e "SELECT COALESCE(SUM(quantity),0) FROM t_usage_ledger WHERE tenant_id=$tenant_id AND metric IN ('storage.source_bytes','storage.artifact_bytes','storage.log_bytes')")
[[ $final_storage_usage == "$baseline_storage_usage" ]] || {
  echo "V2 临时对象清理后存储净用量未恢复: before=$baseline_storage_usage after=$final_storage_usage" >&2
  exit 1
}
cleanup_adjustment_count=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_usage_ledger WHERE tenant_id=$tenant_id AND resource_type='acceptance.cleanup' AND idempotency_key LIKE 'v2-negative-cleanup:%' AND JSON_EXTRACT(metadata,'$.appId')=$app_id")
[[ $cleanup_adjustment_count == 3 ]] || {
  echo "V2 存储用量清理调整数量异常: $cleanup_adjustment_count" >&2
  exit 1
}
remaining_quota_reservations=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_quota_reservation WHERE resource_type='storage' AND resource_id IN ($source_object_id,$logo_object_id,$splash_object_id)")
[[ $remaining_quota_reservations == 0 ]] || {
  echo "V2 临时存储额度预占记录未清理: $remaining_quota_reservations" >&2
  exit 1
}
if [[ -s $object_keys_file ]]; then
  while IFS= read -r object_key; do
    if docker run --rm --network "$network" -e OBJECT_KEY="$object_key" --entrypoint /bin/sh \
      minio/mc:RELEASE.2025-04-16T18-13-26Z -c '
        mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
        mc stat "local/appforge/$OBJECT_KEY" >/dev/null
      ' >/dev/null 2>&1; then
      echo "V2 临时对象未清理: $object_key" >&2
      exit 1
    fi
  done < <(sort -u "$object_keys_file")
fi

mkdir -p "$(dirname "$evidence")"
EVIDENCE_PATH="$evidence" TENANT_ID="$tenant_id" APP_ID="$app_id" VERSION_ID="$version_id" \
PROFILE_ID="$profile_id" PREFLIGHT_ID="$preflight_id" BUILDER_ID="$builder_id" BUILDER_ATTEMPT="$builder_attempt" \
TOOLCHAIN="$toolchain" SOURCE_SHA="$source_sha" REPORT_JSON="$report_json" \
BASELINE_STORAGE_USAGE="$baseline_storage_usage" FINAL_STORAGE_USAGE="$final_storage_usage" \
CLEANUP_ADJUSTMENT_COUNT="$cleanup_adjustment_count" REMAINING_QUOTA_RESERVATIONS="$remaining_quota_reservations" \
ELIGIBLE_BUILDER_COUNT="$eligible_builder_count" ISOLATED_PROJECT="$isolated_project" python3 - <<'PY'
import json, os, pathlib

payload = {
  'schemaVersion': 1,
  'date': '2026-08-15',
  'mode': 'isolated-compose-synthetic-v2-negative-e2e' if os.environ['ISOLATED_PROJECT'] else 'local-development-synthetic-v2-negative-e2e',
  'inputs': {
    'syntheticApkOnly': True,
    'realCustomerDataAccessed': False,
    'productionCredentialsAccessed': False,
    'temporaryIsolatedEnvironment': bool(os.environ['ISOLATED_PROJECT']),
    'sharedDevelopmentDatabaseMutated': not bool(os.environ['ISOLATED_PROJECT']),
  },
  'temporaryIdentity': {
    'tenantId': int(os.environ['TENANT_ID']),
    'appId': int(os.environ['APP_ID']),
    'versionId': int(os.environ['VERSION_ID']),
    'brandingProfileId': int(os.environ['PROFILE_ID']),
    'preflightId': int(os.environ['PREFLIGHT_ID']),
  },
  'worker': {
    'eligibleBuilderCountBeforeWrite': int(os.environ['ELIGIBLE_BUILDER_COUNT']),
    'builderId': os.environ['BUILDER_ID'],
    'builderAttempt': int(os.environ['BUILDER_ATTEMPT']),
    'toolchain': os.environ['TOOLCHAIN'],
  },
  'sourceApkSha256': os.environ['SOURCE_SHA'],
  'preflightReport': json.loads(os.environ['REPORT_JSON']),
  'checks': {
    'managementHttpApiCreatedApplicationAndUploads': 'passed',
    'managementHttpApiCreatedVersionAndBrandingProfile': 'passed',
    'managementHttpApiCreatedPreflight': 'passed',
    'realBuilderClaimedAndInspectedApk': 'passed',
    'missingLauncherAndSplashTargetsReported': 'passed',
    'preflightStatusIncompatible': 'passed',
    'noFormalBuildTaskViaApi': 'passed',
    'noFormalBuildTaskInDatabase': 'passed',
    'temporaryDatabaseRowsRemoved': 'passed',
    'temporaryObjectBytesRemoved': 'passed',
    'storageQuotaNetUsageRestored': 'passed',
  },
  'cleanup': {
    'temporaryBusinessRowsRemaining': 0,
    'objectBytesRemaining': 0,
    'quotaReservationsRemaining': int(os.environ['REMAINING_QUOTA_RESERVATIONS']),
    'storageUsageBefore': int(os.environ['BASELINE_STORAGE_USAGE']),
    'storageUsageAfter': int(os.environ['FINAL_STORAGE_USAGE']),
    'immutableUsageCleanupAdjustments': int(os.environ['CLEANUP_ADJUSTMENT_COUNT']),
    'auditEventsMayRemainForTraceability': True,
    'usageLedgerAdjustmentsRemainForTraceability': True,
  },
}
path=pathlib.Path(os.environ['EVIDENCE_PATH'])
path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
PY
chmod 0600 "$evidence"

echo "V2 负向 E2E 通过：预检 $preflight_id 由 $builder_id 返回 INCOMPATIBLE，正式构建任务为 0，临时数据与对象已清理。"
echo "证据: $evidence"
