#!/usr/bin/env bash

set -euo pipefail

# 只读复核既有 V3 合成验收任务，不创建、修改或删除数据库、MinIO 或 Android 设备状态。
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
old_task_id=${APPFORGE_V3_OLD_TASK_ID:-14}
new_task_id=${APPFORGE_V3_NEW_TASK_ID:-15}
mysql_container=${APPFORGE_V3_MYSQL_CONTAINER:-appforge-mysql}
mysql_database=${APPFORGE_V3_MYSQL_DATABASE:-appforge}
mysql_user=${APPFORGE_V3_MYSQL_USER:-appforge}
mysql_password=${APPFORGE_V3_MYSQL_PASSWORD:-appforge_dev_password}
network=${APPFORGE_DEV_NETWORK:-appforge-dev}
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
go_cache=${APPFORGE_V3_GO_CACHE:-/private/tmp/appforge-v3-go-build-cache}
go_mod_cache=${APPFORGE_V3_GO_MOD_CACHE:-/private/tmp/appforge-v3-go-mod-cache}
evidence=${APPFORGE_V3_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v3-whitelabel-readonly-20260815.json}
expected_package=${APPFORGE_V3_EXPECTED_PACKAGE:-com.appforge.v3runtime20260814}
expected_activity=${APPFORGE_V3_EXPECTED_ACTIVITY:-com.appforge.acceptance.MainActivity}
expected_old_version=${APPFORGE_V3_EXPECTED_OLD_VERSION:-20260820}
expected_new_version=${APPFORGE_V3_EXPECTED_NEW_VERSION:-20260821}
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
trap 'echo "V3 只读复核在第 ${LINENO} 行失败" >&2' ERR

for value in "$old_task_id" "$new_task_id"; do
  [[ $value =~ ^[1-9][0-9]*$ ]] || { echo "V3 验收任务 ID 无效: $value" >&2; exit 1; }
done
[[ $old_task_id != "$new_task_id" ]]
for command in docker python3 shasum; do command -v "$command" >/dev/null || { echo "缺少命令: $command" >&2; exit 1; }; done
for container in "$mysql_container" appforge-minio; do
  container_state=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null)
  [[ $container_state == healthy || $container_state == running ]] || {
    echo "V3 验收依赖容器不健康: $container" >&2
    exit 1
  }
done
docker image inspect "$agent_image" >/dev/null

mysql_scalar() {
  docker exec -e MYSQL_PWD="$mysql_password" "$mysql_container" mysql -u"$mysql_user" -D "$mysql_database" \
    --default-character-set=utf8mb4 -N -B -e "$1"
}

task_metadata() {
  mysql_scalar "SELECT tenant_id,app_id,version_id,signing_config_id,branding_profile_id,branding_revision,white_label_product_id,template_revision,apk_object_id,status,builder_attempt FROM t_build_task WHERE id=$1"
}
read -r old_tenant old_app old_version_id old_signing_id old_branding_id old_branding_revision old_product_id old_template_revision old_output_id old_status old_attempt <<<"$(task_metadata "$old_task_id")"
read -r new_tenant new_app new_version_id new_signing_id new_branding_id new_branding_revision new_product_id new_template_revision new_output_id new_status new_attempt <<<"$(task_metadata "$new_task_id")"
[[ $old_status == SUCCESS && $new_status == SUCCESS && $old_attempt -ge 1 && $new_attempt -ge 1 ]]
[[ $old_tenant == "$new_tenant" && $old_app == "$new_app" && $old_signing_id == "$new_signing_id" ]]
[[ $old_branding_id == "$new_branding_id" && $old_branding_revision == "$new_branding_revision" ]]
[[ $old_product_id == "$new_product_id" && $old_template_revision == "$new_template_revision" ]]
[[ $old_output_id =~ ^[1-9][0-9]*$ && $new_output_id =~ ^[1-9][0-9]*$ ]]

read -r product_tenant product_app current_template_id current_template_revision product_branding_id product_package product_signing_id product_status <<<"$(mysql_scalar "SELECT tenant_id,app_id,template_id,template_revision,branding_profile_id,package_name,signing_config_id,status FROM t_white_label_product WHERE id=$old_product_id")"
[[ $product_tenant == "$old_tenant" && $product_app == "$old_app" && $product_status == 2 ]]
[[ $product_branding_id == "$old_branding_id" && $product_package == "$expected_package" && $product_signing_id == "$old_signing_id" ]]
[[ $current_template_id =~ ^[1-9][0-9]*$ && $current_template_revision == "$old_template_revision" ]]

read -r certificate_sha signing_status <<<"$(mysql_scalar "SELECT certificate_sha256,status FROM t_app_signing_config WHERE id=$old_signing_id AND tenant_id=$old_tenant AND app_id=$old_app")"
[[ $certificate_sha =~ ^[0-9a-f]{64}$ && $signing_status == 1 ]]
read -r binding_certificate binding_signing binding_status first_binding_task last_binding_task <<<"$(mysql_scalar "SELECT certificate_sha256,signing_config_id,status,first_build_task_id,last_build_task_id FROM t_package_certificate_binding WHERE tenant_id=$old_tenant AND package_name='$expected_package'")"
[[ $binding_certificate == "$certificate_sha" && $binding_signing == "$old_signing_id" && $binding_status == 1 ]]
[[ $first_binding_task -gt 0 && $last_binding_task -ge $new_task_id ]]

old_snapshot_b64=$(mysql_scalar "SELECT REPLACE(TO_BASE64(CAST(template_snapshot AS CHAR)),'\\n','') FROM t_build_task WHERE id=$old_task_id")
new_snapshot_b64=$(mysql_scalar "SELECT REPLACE(TO_BASE64(CAST(template_snapshot AS CHAR)),'\\n','') FROM t_build_task WHERE id=$new_task_id")
old_branding_b64=$(mysql_scalar "SELECT REPLACE(TO_BASE64(CAST(branding_snapshot AS CHAR)),'\\n','') FROM t_build_task WHERE id=$old_task_id")
new_branding_b64=$(mysql_scalar "SELECT REPLACE(TO_BASE64(CAST(branding_snapshot AS CHAR)),'\\n','') FROM t_build_task WHERE id=$new_task_id")
OLD_SNAPSHOT_B64="$old_snapshot_b64" NEW_SNAPSHOT_B64="$new_snapshot_b64" \
OLD_BRANDING_B64="$old_branding_b64" NEW_BRANDING_B64="$new_branding_b64" \
PRODUCT_ID="$old_product_id" TARGET_PACKAGE="$expected_package" CERTIFICATE_SHA="$certificate_sha" python3 - <<'PY'
import base64, json, os

decode = lambda key: json.loads(base64.b64decode(os.environ[key]))
old, new = decode('OLD_SNAPSHOT_B64'), decode('NEW_SNAPSHOT_B64')
old_branding, new_branding = decode('OLD_BRANDING_B64'), decode('NEW_BRANDING_B64')
for snapshot in (old, new):
    assert snapshot['productId'] == int(os.environ['PRODUCT_ID'])
    assert snapshot['targetPackageName'] == os.environ['TARGET_PACKAGE']
    assert snapshot['certificateSha256'] == os.environ['CERTIFICATE_SHA']
    assert snapshot['templateRevision'] == 1
    assert len(snapshot['templateChecksum']) == 64
assert old['templateId'] != new['templateId']
assert old['templateCode'].endswith('_old_20260814')
assert new['templateCode'].endswith('_new_20260814')
assert old['templateChecksum'] == new['templateChecksum']
assert json.loads(old['parameterValuesJson'])['runtime']['environment'] == 'clean-old'
assert json.loads(new['parameterValuesJson'])['runtime']['environment'] == 'clean-new'
assert old_branding == new_branding
assert old_branding['profileId'] > 0 and old_branding['revision'] > 0
PY

cross_tenant_count=$(mysql_scalar "SELECT
(SELECT COUNT(*) FROM t_white_label_template t JOIN t_app_application a ON a.id=t.app_id WHERE t.tenant_id<>a.tenant_id) +
(SELECT COUNT(*) FROM t_white_label_template_revision r JOIN t_white_label_template t ON t.id=r.template_id WHERE r.tenant_id<>t.tenant_id) +
(SELECT COUNT(*) FROM t_white_label_product p JOIN t_white_label_template t ON t.id=p.template_id JOIN t_app_branding_profile b ON b.id=p.branding_profile_id JOIN t_app_signing_config s ON s.id=p.signing_config_id WHERE p.tenant_id<>t.tenant_id OR p.tenant_id<>b.tenant_id OR p.tenant_id<>s.tenant_id OR p.app_id<>t.app_id OR p.app_id<>b.app_id OR p.app_id<>s.app_id) +
(SELECT COUNT(*) FROM t_build_task t JOIN t_white_label_product p ON p.id=t.white_label_product_id WHERE t.white_label_product_id>0 AND (t.tenant_id<>p.tenant_id OR t.app_id<>p.app_id))")
[[ $cross_tenant_count == 0 ]] || { echo "发现 V3 跨租户/跨应用引用: $cross_tenant_count" >&2; exit 1; }
missing_snapshot_count=$(mysql_scalar "SELECT COUNT(*) FROM t_build_task WHERE white_label_product_id>0 AND (template_revision<=0 OR template_snapshot IS NULL OR JSON_EXTRACT(template_snapshot,'$.productId')<>white_label_product_id OR JSON_EXTRACT(template_snapshot,'$.templateRevision')<>template_revision)")
[[ $missing_snapshot_count == 0 ]] || { echo "发现 V3 构建缺少不可变快照: $missing_snapshot_count" >&2; exit 1; }
scenario_task_count=$(mysql_scalar "SELECT COUNT(*) FROM t_build_task WHERE id IN (7,8,9,10,11) AND status='SUCCESS' AND white_label_product_id>0 AND template_snapshot IS NOT NULL")
[[ $scenario_task_count == 5 ]] || { echo "V3 历史场景任务不完整: $scenario_task_count/5" >&2; exit 1; }

object_metadata() {
  mysql_scalar "SELECT object_key,sha256,size_bytes,status FROM t_storage_object WHERE id=$1 AND tenant_id=$old_tenant AND app_id=$old_app"
}
old_source_id=$(mysql_scalar "SELECT source_apk_object_id FROM t_app_version WHERE id=$old_version_id AND tenant_id=$old_tenant")
new_source_id=$(mysql_scalar "SELECT source_apk_object_id FROM t_app_version WHERE id=$new_version_id AND tenant_id=$old_tenant")
read -r old_source_key old_source_sha old_source_size old_source_status <<<"$(object_metadata "$old_source_id")"
read -r new_source_key new_source_sha new_source_size new_source_status <<<"$(object_metadata "$new_source_id")"
read -r old_output_key old_output_sha old_output_size old_output_status <<<"$(object_metadata "$old_output_id")"
read -r new_output_key new_output_sha new_output_size new_output_status <<<"$(object_metadata "$new_output_id")"
for key in "$old_source_key" "$new_source_key" "$old_output_key" "$new_output_key"; do
  [[ $key == "tenants/$old_tenant/"* && $key != *..* && $key =~ ^[A-Za-z0-9._/-]+$ ]] || { echo "对象 Key 非法: $key" >&2; exit 1; }
done
for status in "$old_source_status" "$new_source_status" "$old_output_status" "$new_output_status"; do [[ $status == 3 ]]; done

docker run --rm --network "$network" -v "$temporary:/out" \
  -e OLD_SOURCE_KEY="$old_source_key" -e NEW_SOURCE_KEY="$new_source_key" \
  -e OLD_OUTPUT_KEY="$old_output_key" -e NEW_OUTPUT_KEY="$new_output_key" \
  --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c '
set -eu
mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
mc cp "local/appforge/$OLD_SOURCE_KEY" /out/old-source.apk >/dev/null
mc cp "local/appforge/$NEW_SOURCE_KEY" /out/new-source.apk >/dev/null
mc cp "local/appforge/$OLD_OUTPUT_KEY" /out/old-output.apk >/dev/null
mc cp "local/appforge/$NEW_OUTPUT_KEY" /out/new-output.apk >/dev/null
chmod 0644 /out/*.apk
'

verify_file() {
  local path=$1 expected_sha=$2 expected_size=$3
  [[ $(wc -c <"$path" | tr -d ' ') == "$expected_size" ]]
  [[ $(shasum -a 256 "$path" | awk '{print $1}') == "$expected_sha" ]]
}
verify_file "$temporary/old-source.apk" "$old_source_sha" "$old_source_size"
verify_file "$temporary/new-source.apk" "$new_source_sha" "$new_source_size"
verify_file "$temporary/old-output.apk" "$old_output_sha" "$old_output_size"
verify_file "$temporary/new-output.apk" "$new_output_sha" "$new_output_size"

docker run --rm --user 0 -v "$temporary:/data" --entrypoint sh "$agent_image" -lc "
set -eu
for variant in old new; do
  apksigner verify --verbose --print-certs /data/\${variant}-output.apk > /data/\${variant}-signature.txt
  zipalign -c 4 /data/\${variant}-output.apk >/dev/null
  aapt dump badging /data/\${variant}-source.apk > /data/\${variant}-source-badging.txt
  aapt dump badging /data/\${variant}-output.apk > /data/\${variant}-output-badging.txt
  unzip -l /data/\${variant}-output.apk | grep -q 'classes.dex'
  unzip -p /data/\${variant}-output.apk assets/appforge/template.json > /data/\${variant}-template.json
  unzip -p /data/\${variant}-output.apk assets/appforge/customer-runtime.json > /data/\${variant}-runtime.json
  unzip -p /data/\${variant}-output.apk assets/appforge/branding.json > /data/\${variant}-branding.json
  apktool d -f -o /tmp/\${variant}-decoded /data/\${variant}-output.apk >/dev/null 2>&1
  manifest=/tmp/\${variant}-decoded/AndroidManifest.xml
  grep -Fq 'package=\"$expected_package\"' \"\$manifest\"
  grep -Fq 'android:name=\"$expected_package.permission.C2D_MESSAGE\"' \"\$manifest\"
  grep -Fq 'android:name=\"$expected_activity\"' \"\$manifest\"
  grep -Fq 'android:name=\"com.appforge.acceptance.SmokeProvider\"' \"\$manifest\"
  grep -Fq 'android:authorities=\"$expected_package.provider\"' \"\$manifest\"
  grep -Fq 'android:scheme=\"appforgeruntime\"' \"\$manifest\"
  grep -Fq 'android:host=\"runtime.customer.example.com\"' \"\$manifest\"
  ! grep -Eq 'source\\.fileprovider|sourceoauth|source\\.example\\.com' \"\$manifest\"
done
"

apk_field() {
  local file=$1 field=$2
  case "$field" in
    package) sed -n "s/^package: name='\([^']*\)'.*/\1/p" "$file" | head -1 ;;
    versionCode) sed -n "s/^package: name='[^']*' versionCode='\([^']*\)'.*/\1/p" "$file" | head -1 ;;
    versionName) sed -n "s/^package: name='[^']*' versionCode='[^']*' versionName='\([^']*\)'.*/\1/p" "$file" | head -1 ;;
    activity) sed -n "s/^launchable-activity: name='\([^']*\)'.*/\1/p" "$file" | head -1 ;;
    label) sed -n "s/^application-label:'\([^']*\)'.*/\1/p" "$file" | head -1 ;;
  esac
}
[[ $(apk_field "$temporary/old-source-badging.txt" versionCode) == "$expected_old_version" ]]
[[ $(apk_field "$temporary/new-source-badging.txt" versionCode) == "$expected_new_version" ]]
[[ $(apk_field "$temporary/old-output-badging.txt" package) == "$expected_package" ]]
[[ $(apk_field "$temporary/new-output-badging.txt" package) == "$expected_package" ]]
[[ $(apk_field "$temporary/old-output-badging.txt" versionCode) == "$expected_old_version" ]]
[[ $(apk_field "$temporary/new-output-badging.txt" versionCode) == "$expected_new_version" ]]
[[ $(apk_field "$temporary/old-output-badging.txt" activity) == "$expected_activity" ]]
[[ $(apk_field "$temporary/new-output-badging.txt" activity) == "$expected_activity" ]]
[[ $(apk_field "$temporary/old-output-badging.txt" label) == 'AppForge 白标验收' ]]
[[ $(apk_field "$temporary/new-output-badging.txt" label) == 'AppForge 白标验收' ]]
old_apk_certificate=$(sed -n 's/^Signer #1 certificate SHA-256 digest: //p' "$temporary/old-signature.txt" | tr '[:upper:]' '[:lower:]' | head -1)
new_apk_certificate=$(sed -n 's/^Signer #1 certificate SHA-256 digest: //p' "$temporary/new-signature.txt" | tr '[:upper:]' '[:lower:]' | head -1)
[[ $old_apk_certificate == "$certificate_sha" && $new_apk_certificate == "$certificate_sha" ]]

OLD_TEMPLATE="$temporary/old-template.json" NEW_TEMPLATE="$temporary/new-template.json" \
OLD_RUNTIME="$temporary/old-runtime.json" NEW_RUNTIME="$temporary/new-runtime.json" \
OLD_BRANDING="$temporary/old-branding.json" NEW_BRANDING="$temporary/new-branding.json" \
OLD_SNAPSHOT_B64="$old_snapshot_b64" NEW_SNAPSHOT_B64="$new_snapshot_b64" python3 - <<'PY'
import base64, json, os

load = lambda key: json.load(open(os.environ[key], encoding='utf-8'))
decode = lambda key: json.loads(base64.b64decode(os.environ[key]))
old_template, new_template = load('OLD_TEMPLATE'), load('NEW_TEMPLATE')
old_runtime, new_runtime = load('OLD_RUNTIME'), load('NEW_RUNTIME')
old_snapshot, new_snapshot = decode('OLD_SNAPSHOT_B64'), decode('NEW_SNAPSHOT_B64')
for asset, snapshot in ((old_template, old_snapshot), (new_template, new_snapshot)):
    assert asset['schemaVersion'] == 1
    assert asset['productId'] == snapshot['productId']
    assert asset['templateId'] == snapshot['templateId']
    assert asset['templateRevision'] == snapshot['templateRevision']
    assert asset['templateChecksum'] == snapshot['templateChecksum']
    assert asset['targetPackageName'] == snapshot['targetPackageName']
assert old_runtime == {'environment': 'clean-old'}
assert new_runtime == {'environment': 'clean-new'}
assert load('OLD_BRANDING') == load('NEW_BRANDING')
PY

(
  cd "$repo_root/services/core"
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./internal/logic -run 'Test(ValidatePackageName|ValidateRevisionDocumentsRejectsExecutableAndUnsafePath|ValidateParameterValuesEnforcesSchema|SensitiveParameterSchemaAndEmbeddedSecretPolicy|RevisionOperationSchemaAndParameterReferences)$' -count=1
)
(
  cd "$repo_root/services/builder"
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./internal/worker -run 'Test(ApplyWhiteLabelTemplate|ProtectManifestComponentNamesAcrossPackageRewrite|DecodeWhiteLabelBuildSnapshotRejectsIncompleteSnapshot|DecryptSensitiveWhiteLabelParameters)$' -count=1
)
(
  cd "$repo_root/appforge-api"
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./internal/logic -run 'TestMask(SealedJSON|TemplateSnapshot)$' -count=1
)

mkdir -p "$(dirname "$evidence")"
EVIDENCE_PATH="$evidence" OLD_TASK_ID="$old_task_id" NEW_TASK_ID="$new_task_id" \
OLD_OUTPUT_SHA="$old_output_sha" NEW_OUTPUT_SHA="$new_output_sha" PACKAGE_NAME="$expected_package" \
CERTIFICATE_SHA="$certificate_sha" OLD_VERSION="$expected_old_version" NEW_VERSION="$expected_new_version" \
SCENARIO_TASK_COUNT="$scenario_task_count" python3 - <<'PY'
import json, os, pathlib

payload = {
  'schemaVersion': 1,
  'date': '2026-08-15',
  'mode': 'read-only-existing-synthetic-v3-evidence',
  'databaseMutated': False,
  'objectStorageMutated': False,
  'oldTaskId': int(os.environ['OLD_TASK_ID']),
  'newTaskId': int(os.environ['NEW_TASK_ID']),
  'oldOutputSha256': os.environ['OLD_OUTPUT_SHA'],
  'newOutputSha256': os.environ['NEW_OUTPUT_SHA'],
  'packageName': os.environ['PACKAGE_NAME'],
  'certificateSha256': os.environ['CERTIFICATE_SHA'],
  'versionCodeUpgrade': [int(os.environ['OLD_VERSION']), int(os.environ['NEW_VERSION'])],
  'historicalScenarioTaskCount': int(os.environ['SCENARIO_TASK_COUNT']),
  'checks': {
    'tenantAndAppReferenceIntegrity': 'passed',
    'immutableTaskSnapshots': 'passed',
    'historicalTemplateChecksumTraceability': 'passed',
    'packageCertificateBinding': 'passed',
    'sourceAndOutputObjectIntegrity': 'passed',
    'dynamicPackageName': 'passed',
    'relativeComponentClassProtection': 'passed',
    'permissionAndProviderAuthority': 'passed',
    'oauthSchemeAndAppLinkHost': 'passed',
    'versionCodeUpgradeOrder': 'passed',
    'sameSigningCertificate': 'passed',
    'classesDexAndLaunchableActivity': 'passed',
    'brandingAssetContinuity': 'passed',
    'templateAndExtensionAssets': 'passed',
    'schemaPathScriptSecretNegativeUnitTests': 'passed',
  },
  'limitations': [
    'This verifier is read-only and relies on existing synthetic V3 tasks 7-15.',
    'It statically re-verifies APKs but does not mutate or launch an Android device.',
    'Install, first-launch and in-place upgrade evidence remains the recorded 2026-08-14 API 29 emulator run.',
    'Wrong-certificate and malformed-template API flows are not recreated; current negative evidence is code-level tests plus historical records.',
    'V2 criterion 3 remains in closeout, so the V3 prerequisite chain is not yet release-complete.',
  ],
}
path = pathlib.Path(os.environ['EVIDENCE_PATH'])
path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + '\n')
PY
chmod 0600 "$evidence"

echo "V3 高级白标只读复核通过：任务 ${old_task_id}→${new_task_id}，包名 ${expected_package}，versionCode ${expected_old_version}→${expected_new_version}。"
echo "限制：本次未操作 Android 设备；安装、首次启动和覆盖升级沿用 2026-08-14 API 29 Emulator 历史证据。"
echo "证据: $evidence"
