#!/usr/bin/env bash

set -euo pipefail

image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
synthetic_payload_mib=${APPFORGE_LOCAL_AGENT_SYNTHETIC_PAYLOAD_MIB:-0}

[[ $synthetic_payload_mib =~ ^[0-9]+$ ]] && ((synthetic_payload_mib <= 256)) || {
  echo "验收失败: APPFORGE_LOCAL_AGENT_SYNTHETIC_PAYLOAD_MIB 必须是 0 到 256 的整数" >&2
  exit 1
}

docker run --rm --entrypoint sh \
  -e APPFORGE_LOCAL_AGENT_SYNTHETIC_PAYLOAD_MIB="$synthetic_payload_mib" \
  "$image" -lc '
set -eu

task_root=$(mktemp -d /tmp/appforge-local-executor.XXXXXX)
trap '\''rm -rf "$task_root"'\'' EXIT
chmod 0700 "$task_root"

manifest_path="$task_root/AndroidManifest.xml"
source_path="$task_root/source.apk"
keystore_path="$task_root/release.jks"
certificate_path="$task_root/release.der"
task_path="$task_root/task.json"
result_path="$task_root/result.json"
assets_path="$task_root/assets"
payload_mib=${APPFORGE_LOCAL_AGENT_SYNTHETIC_PAYLOAD_MIB:-0}

printf '\''%s\n'\'' \
  '\''<?xml version="1.0" encoding="utf-8"?>'\'' \
  '\''<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.local" android:versionCode="1" android:versionName="1.0">'\'' \
  '\''  <uses-sdk android:minSdkVersion="21" />'\'' \
  '\''  <application android:allowBackup="false" android:label="Local Acceptance" />'\'' \
  '\''</manifest>'\'' >"$manifest_path"

if [ "$payload_mib" -gt 0 ]; then
  mkdir -p "$assets_path"
  dd if=/dev/urandom of="$assets_path/appforge-synthetic-payload.bin" bs=1048576 count="$payload_mib" 2>/dev/null
  chmod 0600 "$assets_path/appforge-synthetic-payload.bin"
  payload_sha=$(sha256sum "$assets_path/appforge-synthetic-payload.bin" | cut -d " " -f 1)
  aapt package -f -M "$manifest_path" -I /usr/share/android-framework-res/framework-res.apk \
    -A "$assets_path" -0 bin -F "$source_path"
  source_payload_sha=$(unzip -p "$source_path" assets/appforge-synthetic-payload.bin | sha256sum | cut -d " " -f 1)
  [ "$source_payload_sha" = "$payload_sha" ]
else
  aapt package -f -M "$manifest_path" -I /usr/share/android-framework-res/framework-res.apk -F "$source_path"
fi
keytool -genkeypair -noprompt -alias release -keyalg RSA -keysize 2048 -validity 3650 \
  -dname "CN=AppForge Local Acceptance,O=AppForge,C=CN" \
  -keystore "$keystore_path" -storepass changeit -keypass changeit >/dev/null 2>&1
keytool -exportcert -alias release -keystore "$keystore_path" -storepass changeit -file "$certificate_path" >/dev/null 2>&1

chmod 0600 "$manifest_path" "$source_path" "$keystore_path" "$certificate_path"
source_size=$(stat -c %s "$source_path")
minimum_payload_bytes=$((payload_mib * 1048576))
[ "$source_size" -ge "$minimum_payload_bytes" ]
keystore_size=$(stat -c %s "$keystore_path")
source_sha=$(sha256sum "$source_path" | cut -d " " -f 1)
keystore_sha=$(sha256sum "$keystore_path" | cut -d " " -f 1)
certificate_sha=$(sha256sum "$certificate_path" | cut -d " " -f 1)

printf '\''{"task":{"id":7001,"tenant_id":1001,"app_id":2001,"version_id":3001,"builder_attempt":1,"channel_code":"local-acceptance","version_code":1,"version_name":"1.0"},"artifactMode":1,"bundle":{"schema_version":3,"task":{"id":7001,"tenant_id":1001,"app_id":2001,"version_id":3001,"builder_attempt":1,"channel_code":"local-acceptance","version_code":1,"version_name":"1.0"},"package_name":"com.example.local","api_host":"https://api.example.com","channel_name":"Local Acceptance","landing_url":"https://example.com","key_alias":"release","signer_certificate_sha256":"%s","branding_snapshot_json":"","template_snapshot_json":"","inputs":[{"role":"source_apk","object_id":4001,"object_type":1,"original_name":"source.apk","content_type":"application/vnd.android.package-archive","size_bytes":%s,"sha256":"%s","local_path":"%s"},{"role":"keystore","object_id":4002,"object_type":2,"original_name":"release.jks","content_type":"application/octet-stream","size_bytes":%s,"sha256":"%s","local_path":"%s"}],"blocked_reason":""}}'\'' \
  "$certificate_sha" "$source_size" "$source_sha" "$source_path" \
  "$keystore_size" "$keystore_sha" "$keystore_path" >"$task_path"
chmod 0600 "$task_path"

APPFORGE_KEYSTORE_PASSWORD=changeit APPFORGE_KEY_PASSWORD=changeit \
  appforge-local-build --task "$task_path" --result "$result_path"

test -s "$result_path"
test -s "$task_root/channel.apk"
test -s "$task_root/build.log"
channel_size=$(stat -c %s "$task_root/channel.apk")
[ "$channel_size" -ge "$minimum_payload_bytes" ]
if [ "$payload_mib" -gt 0 ]; then
  channel_payload_sha=$(unzip -p "$task_root/channel.apk" assets/appforge-synthetic-payload.bin | sha256sum | cut -d " " -f 1)
  [ "$channel_payload_sha" = "$payload_sha" ]
fi
grep -q '\''"apkSha256"'\'' "$result_path"
if grep -q '\''"error"'\'' "$result_path"; then
  echo "本地执行器返回错误: $(cat "$result_path")" >&2
  exit 1
fi
apksigner verify --verbose --print-certs "$task_root/channel.apk" >/dev/null
aapt dump badging "$task_root/channel.apk" | grep -q "package: name='\''com.example.local'\''"

echo "通过: Local Agent 固定执行器完成 APK 渠道注入、对齐、签名和结果摘要校验，合成载荷=${payload_mib}MiB"
'
