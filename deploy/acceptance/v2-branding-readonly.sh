#!/usr/bin/env bash

set -euo pipefail

# 只读复核既有 V2 合成验收任务，不创建、修改或删除数据库记录。
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
task_id=${APPFORGE_V2_ACCEPTANCE_TASK_ID:-4}
mysql_container=${APPFORGE_V2_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V2_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V2_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V2_MYSQL_PASSWORD:-appforge_dev_password}
network=${APPFORGE_DEV_NETWORK:-appforge-dev}
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
go_cache=${APPFORGE_V2_GO_CACHE:-/private/tmp/appforge-go-build-cache}
go_mod_cache=${APPFORGE_V2_GO_MOD_CACHE:-/private/tmp/appforge-go-mod-cache}
evidence=${APPFORGE_V2_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v2-branding-readonly-20260815.json}
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
trap 'echo "V2 只读复核在第 ${LINENO} 行失败" >&2' ERR

[[ $task_id =~ ^[1-9][0-9]*$ ]] || { echo 'V2 验收任务 ID 无效' >&2; exit 1; }
for command in docker python3 shasum; do command -v "$command" >/dev/null || { echo "缺少命令: $command" >&2; exit 1; }; done
for container in "$mysql_container" appforge-core-rpc appforge-builder-rpc appforge-admin-api; do
  [[ $(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null) == healthy ]] || {
    echo "V2 验收依赖容器不健康: $container" >&2
    exit 1
  }
done
[[ $(docker ps --format '{{.Names}}' | grep -c '^appforge-builder-worker-') -ge 1 ]] || { echo '没有运行中的 Builder Worker' >&2; exit 1; }

mysql_scalar() {
  docker exec -e MYSQL_PWD="$mysql_password" "$mysql_container" mysql -u"$mysql_user" -D "$mysql_database" \
    --default-character-set=utf8mb4 -N -B -e "$1"
}

eligible_branding_builders=$(mysql_scalar "SELECT COUNT(*) FROM t_builder_node
WHERE status=1 AND drain_status=1 AND last_heartbeat_at>=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 90 SECOND)
  AND max_concurrency>running_count AND disk_capacity>0
  AND disk_free>=536870912 AND disk_free*100>=disk_capacity*2
  AND build_protocol_version>=1 AND toolchain_version<>''
  AND JSON_VALID(capability_json) AND JSON_EXTRACT(capability_json,'$.branding')=TRUE")
[[ $eligible_branding_builders =~ ^[0-9]+$ ]]

read -r tenant_id app_id version_id profile_id branding_revision output_object_id task_status builder_attempt <<<"$(mysql_scalar "SELECT tenant_id,app_id,version_id,branding_profile_id,branding_revision,apk_object_id,status,builder_attempt FROM t_build_task WHERE id=$task_id")"
[[ $tenant_id =~ ^[1-9][0-9]*$ && $profile_id =~ ^[1-9][0-9]*$ && $output_object_id =~ ^[1-9][0-9]*$ && $task_status == SUCCESS ]] || {
  echo '指定任务不是完整的 V2 成功任务' >&2
  exit 1
}

read -r profile_tenant profile_app logo_object_id splash_object_id profile_status profile_revision <<<"$(mysql_scalar "SELECT tenant_id,app_id,logo_object_id,splash_object_id,status,revision FROM t_app_branding_profile WHERE id=$profile_id")"
[[ $profile_tenant == "$tenant_id" && $profile_app == "$app_id" && $profile_status == 2 && $profile_revision -ge "$branding_revision" ]]
profile_name=$(mysql_scalar "SELECT app_name FROM t_app_branding_profile WHERE id=$profile_id")
profile_api_host=$(mysql_scalar "SELECT api_host FROM t_app_branding_profile WHERE id=$profile_id")
package_name=$(mysql_scalar "SELECT a.package_name FROM t_app_version v JOIN t_app_application a ON a.id=v.app_id AND a.tenant_id=v.tenant_id WHERE v.id=$version_id AND v.tenant_id=$tenant_id")

snapshot_b64=$(mysql_scalar "SELECT REPLACE(TO_BASE64(CAST(branding_snapshot AS CHAR)),'\\n','') FROM t_build_task WHERE id=$task_id")
preflight_b64=$(mysql_scalar "SELECT REPLACE(TO_BASE64(CAST(report_json AS CHAR)),'\\n','') FROM t_branding_preflight WHERE tenant_id=$tenant_id AND branding_profile_id=$profile_id AND branding_revision=$branding_revision AND version_id=$version_id AND status=2 LIMIT 1")
[[ -n $snapshot_b64 && -n $preflight_b64 ]] || { echo '缺少品牌快照或兼容预检证据' >&2; exit 1; }
SNAPSHOT_B64="$snapshot_b64" PREFLIGHT_B64="$preflight_b64" PROFILE_ID="$profile_id" REVISION="$branding_revision" \
PROFILE_NAME="$profile_name" API_HOST="$profile_api_host" python3 - <<'PY'
import base64, json, os
snapshot=json.loads(base64.b64decode(os.environ['SNAPSHOT_B64']))
report=json.loads(base64.b64decode(os.environ['PREFLIGHT_B64']))
assert snapshot['profileId'] == int(os.environ['PROFILE_ID'])
assert snapshot['revision'] == int(os.environ['REVISION'])
assert snapshot['appName'] == os.environ['PROFILE_NAME']
assert snapshot['apiHost'] == os.environ['API_HOST']
assert report['schemaVersion'] == 1 and report['compatible'] is True
checks={item['name']: item for item in report['checks']}
for name in ('apktool_decode','application_label','launcher_icon','splash_resource','apktool_rebuild'):
    assert checks[name]['passed'] is True and checks[name]['message']
PY

cross_tenant_count=$(mysql_scalar "SELECT
(SELECT COUNT(*) FROM t_app_branding_profile p JOIN t_storage_object l ON l.id=p.logo_object_id JOIN t_storage_object s ON s.id=p.splash_object_id WHERE l.tenant_id<>p.tenant_id OR s.tenant_id<>p.tenant_id OR l.app_id<>p.app_id OR s.app_id<>p.app_id OR l.object_type<>5 OR s.object_type<>6) +
(SELECT COUNT(*) FROM t_branding_preflight f JOIN t_app_branding_profile p ON p.id=f.branding_profile_id JOIN t_app_version v ON v.id=f.version_id WHERE f.tenant_id<>p.tenant_id OR f.tenant_id<>v.tenant_id OR f.app_id<>p.app_id OR f.app_id<>v.app_id) +
(SELECT COUNT(*) FROM t_build_task t JOIN t_app_branding_profile p ON p.id=t.branding_profile_id WHERE t.branding_profile_id>0 AND (t.tenant_id<>p.tenant_id OR t.app_id<>p.app_id))")
[[ $cross_tenant_count == 0 ]] || { echo "发现品牌跨租户/跨应用引用: $cross_tenant_count" >&2; exit 1; }
snapshot_count=$(mysql_scalar "SELECT COUNT(*) FROM t_build_task WHERE branding_profile_id>0 AND (branding_revision<=0 OR branding_snapshot IS NULL OR JSON_EXTRACT(branding_snapshot,'$.profileId')<>branding_profile_id OR JSON_EXTRACT(branding_snapshot,'$.revision')<>branding_revision)")
[[ $snapshot_count == 0 ]] || { echo "发现品牌构建缺少不可变快照: $snapshot_count" >&2; exit 1; }
recovered_branding_tasks=$(mysql_scalar "SELECT COUNT(*) FROM t_build_task WHERE branding_profile_id>0 AND status='SUCCESS' AND builder_attempt>1")
[[ $recovered_branding_tasks -ge 1 ]] || { echo '缺少恢复后成功的品牌构建任务证据' >&2; exit 1; }

object_metadata() {
  mysql_scalar "SELECT object_key,sha256,size_bytes,status FROM t_storage_object WHERE id=$1 AND tenant_id=$tenant_id AND app_id=$app_id"
}
source_object_id=$(mysql_scalar "SELECT source_apk_object_id FROM t_app_version WHERE id=$version_id AND tenant_id=$tenant_id")
read -r source_key source_sha source_size source_status <<<"$(object_metadata "$source_object_id")"
read -r logo_key logo_sha logo_size logo_status <<<"$(object_metadata "$logo_object_id")"
read -r splash_key splash_sha splash_size splash_status <<<"$(object_metadata "$splash_object_id")"
read -r output_key output_sha output_size output_status <<<"$(object_metadata "$output_object_id")"
for key in "$source_key" "$logo_key" "$splash_key" "$output_key"; do
  [[ $key == "tenants/$tenant_id/"* && $key != *..* && $key =~ ^[A-Za-z0-9._/-]+$ ]] || { echo "对象 Key 非法: $key" >&2; exit 1; }
done
[[ $source_status == 3 && $logo_status == 3 && $splash_status == 3 && $output_status == 3 ]]

docker run --rm --network "$network" -v "$temporary:/out" \
  -e SOURCE_KEY="$source_key" -e LOGO_KEY="$logo_key" -e SPLASH_KEY="$splash_key" -e OUTPUT_KEY="$output_key" \
  --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c '
set -eu
mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
mc cp "local/appforge/$SOURCE_KEY" /out/source.apk >/dev/null
mc cp "local/appforge/$LOGO_KEY" /out/logo.png >/dev/null
mc cp "local/appforge/$SPLASH_KEY" /out/splash.png >/dev/null
mc cp "local/appforge/$OUTPUT_KEY" /out/output.apk >/dev/null
chmod 0644 /out/*
'

verify_file() {
  local path=$1 expected_sha=$2 expected_size=$3
  [[ $(wc -c <"$path" | tr -d ' ') == "$expected_size" ]]
  [[ $(shasum -a 256 "$path" | awk '{print $1}') == "$expected_sha" ]]
}
verify_file "$temporary/source.apk" "$source_sha" "$source_size"
verify_file "$temporary/logo.png" "$logo_sha" "$logo_size"
verify_file "$temporary/splash.png" "$splash_sha" "$splash_size"
verify_file "$temporary/output.apk" "$output_sha" "$output_size"

docker run --rm --user 0 -v "$temporary:/data:ro" --entrypoint sh "$agent_image" -lc '
set -eu
apksigner verify --verbose --print-certs /data/output.apk >/tmp/signature.txt
zipalign -c 4 /data/output.apk >/dev/null
apktool d -f -o /tmp/source /data/source.apk >/dev/null 2>&1
apktool d -f -o /tmp/output /data/output.apk >/dev/null 2>&1
source_icon=$(find /tmp/source/res -type f \( -path "*/mipmap*/ic_launcher.png" -o -path "*/mipmap*/ic_launcher.webp" \) | sort | head -1)
output_icon=$(find /tmp/output/res -type f \( -path "*/mipmap*/ic_launcher.png" -o -path "*/mipmap*/ic_launcher.webp" \) | sort | head -1)
source_splash=$(find /tmp/source/res -type f \( -path "*/drawable*/splash_logo.png" -o -path "*/drawable*/splash_logo.webp" \) | sort | head -1)
output_splash=$(find /tmp/output/res -type f \( -path "*/drawable*/splash_logo.png" -o -path "*/drawable*/splash_logo.webp" \) | sort | head -1)
test -n "$source_icon" -a -n "$output_icon" -a -n "$source_splash" -a -n "$output_splash"
! cmp -s "$source_icon" "$output_icon"
! cmp -s "$source_splash" "$output_splash"
'

source_package=$(docker run --rm --user 0 -v "$temporary:/data:ro" --entrypoint sh "$agent_image" -lc "aapt dump badging /data/source.apk | sed -n \"s/^package: name='\\([^']*\\)'.*/\\1/p\" | head -1")
output_package=$(docker run --rm --user 0 -v "$temporary:/data:ro" --entrypoint sh "$agent_image" -lc "aapt dump badging /data/output.apk | sed -n \"s/^package: name='\\([^']*\\)'.*/\\1/p\" | head -1")
output_label=$(docker run --rm --user 0 -v "$temporary:/data:ro" --entrypoint sh "$agent_image" -lc "aapt dump badging /data/output.apk | sed -n \"s/^application-label:'\\([^']*\\)'.*/\\1/p\" | head -1")
certificate_sha=$(docker run --rm --user 0 -v "$temporary:/data:ro" --entrypoint sh "$agent_image" -lc "apksigner verify --print-certs /data/output.apk | sed -n 's/^Signer #1 certificate SHA-256 digest: //p' | tr '[:upper:]' '[:lower:]' | head -1")
[[ $source_package == "$package_name" && $output_package == "$package_name" && $output_label == "$profile_name" && $certificate_sha =~ ^[0-9a-f]{64}$ ]]

docker run --rm --user 0 -v "$temporary:/data:ro" --entrypoint sh "$agent_image" -lc 'unzip -p /data/output.apk assets/appforge/branding.json' >"$temporary/branding.json"
docker run --rm --user 0 -v "$temporary:/data:ro" --entrypoint sh "$agent_image" -lc 'unzip -p /data/output.apk assets/appforge/channel.json' >"$temporary/channel.json"
BRANDING="$temporary/branding.json" CHANNEL="$temporary/channel.json" PROFILE_ID="$profile_id" REVISION="$branding_revision" \
PROFILE_NAME="$profile_name" API_HOST="$profile_api_host" LOGO_SHA="$logo_sha" SPLASH_SHA="$splash_sha" TASK_ID="$task_id" python3 - <<'PY'
import json, os
branding=json.load(open(os.environ['BRANDING']))
channel=json.load(open(os.environ['CHANNEL']))
assert branding == {
    'schemaVersion': 1, 'profileId': int(os.environ['PROFILE_ID']), 'revision': int(os.environ['REVISION']),
    'appName': os.environ['PROFILE_NAME'], 'apiHost': os.environ['API_HOST'],
    'logoSha256': os.environ['LOGO_SHA'], 'splashSha256': os.environ['SPLASH_SHA'],
}
assert channel['buildTaskId'] == int(os.environ['TASK_ID']) and channel['apiHost'] == os.environ['API_HOST']
PY

(
  cd "$repo_root/appforge-api"
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./internal/logic -run 'TestVerifyBrandingImageBoundaries' -count=1
)
(
  cd "$repo_root/services/core"
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./internal/logic -run 'Test(ValidateBranding|ParseBranding|BrandingPreflightNodeEligible)' -count=1
)
(
  cd "$repo_root/services/builder"
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./internal/worker -run 'Test(DecodeBuildBranding|BrandingPreflight)' -count=1
)

mkdir -p "$(dirname "$evidence")"
EVIDENCE_PATH="$evidence" TASK_ID="$task_id" OUTPUT_SHA="$output_sha" PACKAGE_NAME="$output_package" APP_NAME="$output_label" \
CERTIFICATE_SHA="$certificate_sha" RECOVERED_TASKS="$recovered_branding_tasks" \
ELIGIBLE_BRANDING_BUILDERS="$eligible_branding_builders" python3 - <<'PY'
import json, os, pathlib
eligible_builders=int(os.environ['ELIGIBLE_BRANDING_BUILDERS'])
limitations=[
  'This verifier is read-only and relies on the existing synthetic V2 task selected by APPFORGE_V2_ACCEPTANCE_TASK_ID.',
  'It does not create an incompatible APK through the public API and therefore does not close V2 criterion 3 by itself.',
  'Android installation/start evidence remains the previously recorded V3 API 29 emulator run.',
]
if eligible_builders == 0:
  limitations.append('No eligible branding Builder was available; V4 disk/node recovery is required before the write E2E can start.')
payload={
  'schemaVersion': 1,
  'date': '2026-08-15',
  'mode': 'read-only-existing-synthetic-v2-evidence',
  'databaseMutated': False,
  'taskId': int(os.environ['TASK_ID']),
  'outputSha256': os.environ['OUTPUT_SHA'],
  'packageName': os.environ['PACKAGE_NAME'],
  'appName': os.environ['APP_NAME'],
  'certificateSha256': os.environ['CERTIFICATE_SHA'],
  'recoveredBrandingTaskCount': int(os.environ['RECOVERED_TASKS']),
  'eligibleBrandingBuilderCountAtReview': eligible_builders,
  'checks': {
    'tenantAndAppReferenceIntegrity': 'passed', 'compatiblePreflightReport': 'passed',
    'successfulBuild': 'passed', 'packageNameUnchanged': 'passed', 'applicationLabel': 'passed',
    'logoAndSplashResourcesChanged': 'passed', 'brandingSnapshotAsset': 'passed',
    'apiHostAndRevision': 'passed', 'apkSignature': 'passed', 'imageAndMetadataNegativeUnitTests': 'passed',
    'locatablePreflightFailureUnitTest': 'passed', 'recoveredBrandingTaskEvidence': 'passed',
    'preflightBuilderSchedulingGateUnitTest': 'passed',
  },
  'limitations': limitations,
}
path=pathlib.Path(os.environ['EVIDENCE_PATH'])
path.write_text(json.dumps(payload,ensure_ascii=False,indent=2)+'\n')
PY
chmod 0600 "$evidence"

echo "V2 基础白标只读复核通过：任务 ${task_id}，APK ${output_sha}。"
echo "限制：尚未通过公共 API 创建并清理不兼容 APK 预检，因此 V2 验收标准 3 仍需独立 E2E。"
echo "证据: $evidence"
