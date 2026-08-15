#!/bin/sh

set -eu

task_file=
result_file=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --task) task_file=$2; shift 2 ;;
    --result) result_file=$2; shift 2 ;;
    *) echo "unexpected executor argument: $1" >&2; exit 2 ;;
  esac
done
[ -n "$task_file" ] && [ -n "$result_file" ]

attempt=$(sed -n 's/.*"builder_attempt"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$task_file" | head -n 1)
[ -n "$attempt" ]
root=$(dirname "$task_file")
manifest_path="$root/AndroidManifest.xml"
source_path="$root/source.apk"
keystore_path="$root/release.jks"
certificate_path="$root/release.der"

printf '%s\n' \
  '<?xml version="1.0" encoding="utf-8"?>' \
  '<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.local" android:versionCode="1" android:versionName="1.0">' \
  '  <uses-sdk android:minSdkVersion="21" />' \
  '  <application android:allowBackup="false" android:label="Recovery Acceptance" />' \
  '</manifest>' >"$manifest_path"
aapt package -f -M "$manifest_path" -I /usr/share/android-framework-res/framework-res.apk -F "$source_path"
keytool -genkeypair -noprompt -alias release -keyalg RSA -keysize 2048 -validity 3650 \
  -dname "CN=AppForge Recovery Acceptance,O=AppForge,C=CN" \
  -keystore "$keystore_path" -storepass changeit -keypass changeit >/dev/null 2>&1
keytool -exportcert -alias release -keystore "$keystore_path" -storepass changeit -file "$certificate_path" >/dev/null 2>&1
chmod 0600 "$manifest_path" "$source_path" "$keystore_path" "$certificate_path"

source_size=$(stat -c %s "$source_path")
keystore_size=$(stat -c %s "$keystore_path")
source_sha=$(sha256sum "$source_path" | cut -d ' ' -f 1)
keystore_sha=$(sha256sum "$keystore_path" | cut -d ' ' -f 1)
certificate_sha=$(sha256sum "$certificate_path" | cut -d ' ' -f 1)
printf '{"task":{"id":7001,"tenant_id":1001,"app_id":2001,"version_id":3001,"builder_attempt":%s,"channel_code":"recovery-acceptance","version_code":1,"version_name":"1.0"},"artifactMode":1,"bundle":{"schema_version":3,"task":{"id":7001,"tenant_id":1001,"app_id":2001,"version_id":3001,"builder_attempt":%s,"channel_code":"recovery-acceptance","version_code":1,"version_name":"1.0"},"package_name":"com.example.local","api_host":"https://api.example.com","channel_name":"Recovery Acceptance","landing_url":"https://example.com","key_alias":"release","signer_certificate_sha256":"%s","branding_snapshot_json":"","template_snapshot_json":"","inputs":[{"role":"source_apk","object_id":4001,"object_type":1,"original_name":"source.apk","content_type":"application/vnd.android.package-archive","size_bytes":%s,"sha256":"%s","local_path":"%s"},{"role":"keystore","object_id":4002,"object_type":2,"original_name":"release.jks","content_type":"application/octet-stream","size_bytes":%s,"sha256":"%s","local_path":"%s"}],"blocked_reason":""}}' \
  "$attempt" "$attempt" "$certificate_sha" "$source_size" "$source_sha" "$source_path" \
  "$keystore_size" "$keystore_sha" "$keystore_path" >"$task_file"
chmod 0600 "$task_file"

touch "/acceptance/attempt-${attempt}.started"
if [ "$attempt" = 1 ]; then
  while [ ! -f /acceptance/release-attempt-1 ]; do sleep 1; done
fi

appforge-local-build --task "$task_file" --result "$result_file"
apk_path="$root/channel.apk"
log_path="$root/build.log"
apk_sha=$(sha256sum "$apk_path" | cut -d ' ' -f 1)
log_sha=$(sha256sum "$log_path" | cut -d ' ' -f 1)
apk_size=$(stat -c %s "$apk_path")
log_size=$(stat -c %s "$log_path")
cp "$apk_path" "/acceptance/channel-attempt-${attempt}.apk"
chmod 0600 "/acceptance/channel-attempt-${attempt}.apk"
printf '{"apkPath":"%s","apkReference":"local-agent://artifacts/7001/%s/channel.apk","apkSha256":"%s","apkSize":%s,"logPath":"%s","logReference":"local-agent://artifacts/7001/%s/build.log","logSha256":"%s","logSize":%s}' \
  "$apk_path" "$attempt" "$apk_sha" "$apk_size" "$log_path" "$attempt" "$log_sha" "$log_size" >"$result_file"
chmod 0600 "$result_file"
touch "/acceptance/attempt-${attempt}.completed"
