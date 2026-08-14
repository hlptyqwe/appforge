#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 7 ]]; then
  echo "用法: $0 <旧版APK> <新版APK> <期望包名> <期望启动Activity> <旧versionCode> <新versionCode> <期望证书SHA-256>" >&2
  exit 2
fi

old_apk=$1
new_apk=$2
expected_package=$3
expected_activity=$4
old_version_code=$5
new_version_code=$6
expected_certificate=$(tr '[:upper:]' '[:lower:]' <<<"$7")

for apk_file in "$old_apk" "$new_apk"; do
  if [[ ! -f "$apk_file" ]]; then
    echo "APK不存在: $apk_file" >&2
    exit 2
  fi
done

resolve_android_tool() {
  local tool_name=$1
	local required=${2:-true}
  local sdk_root=${ANDROID_SDK_ROOT:-${ANDROID_HOME:-}}
  local resolved_path

  if resolved_path=$(command -v "$tool_name" 2>/dev/null); then
    printf '%s\n' "$resolved_path"
    return
  fi
  if [[ -n "$sdk_root" && "$tool_name" == "adb" && -x "$sdk_root/platform-tools/adb" ]]; then
    printf '%s\n' "$sdk_root/platform-tools/adb"
    return
  fi
  if [[ -n "$sdk_root" ]]; then
    resolved_path=$(find "$sdk_root/build-tools" -type f -name "$tool_name" -perm -111 2>/dev/null | sort | tail -1)
    if [[ -n "$resolved_path" ]]; then
      printf '%s\n' "$resolved_path"
      return
    fi
  fi
	if [[ "$required" == "true" ]]; then
		echo "未找到Android工具: ${tool_name}；请配置PATH或ANDROID_SDK_ROOT" >&2
		exit 2
	fi
	return 1
}

adb_bin=$(resolve_android_tool adb)

device_count=$("$adb_bin" devices | awk 'NR > 1 && $2 == "device" { count++ } END { print count + 0 }')
if [[ "$device_count" -ne 1 ]]; then
  echo "必须且只能连接一台已授权Android设备，当前为 $device_count 台：" >&2
  "$adb_bin" devices -l >&2
  exit 3
fi

aapt_bin=$(resolve_android_tool aapt false || true)
apksigner_bin=$(resolve_android_tool apksigner false || true)

apk_field() {
  local apk_file=$1
  local field_name=$2
	case "$field_name" in
	name)
		"$aapt_bin" dump badging "$apk_file" | sed -n "s/^package: name='\([^']*\)'.*/\1/p" | head -1
		;;
	versionCode)
		"$aapt_bin" dump badging "$apk_file" | sed -n "s/^package: name='[^']*' versionCode='\([^']*\)'.*/\1/p" | head -1
		;;
	*)
		echo "不支持的APK字段: $field_name" >&2
		exit 2
		;;
	esac
}

apk_launchable_activity() {
  "$aapt_bin" dump badging "$1" | sed -n "s/^launchable-activity: name='\([^']*\)'.*/\1/p" | head -1
}

apk_certificate() {
  "$apksigner_bin" verify --print-certs "$1" |
    sed -n 's/^Signer #1 certificate SHA-256 digest: //p' |
    tr '[:upper:]' '[:lower:]' |
    head -1
}

if (( new_version_code <= old_version_code )); then
  echo "新版versionCode必须递增: old=$old_version_code new=$new_version_code" >&2
  exit 4
fi

if [[ -n "$aapt_bin" ]]; then
	old_package=$(apk_field "$old_apk" name)
	new_package=$(apk_field "$new_apk" name)
	old_apk_version_code=$(apk_field "$old_apk" versionCode)
	new_apk_version_code=$(apk_field "$new_apk" versionCode)
	old_activity=$(apk_launchable_activity "$old_apk")
	new_activity=$(apk_launchable_activity "$new_apk")
	if [[ "$old_package" != "$expected_package" || "$new_package" != "$expected_package" ]]; then
		echo "包名不一致: old=$old_package new=$new_package expected=$expected_package" >&2
		exit 4
	fi
	if [[ "$old_activity" != "$expected_activity" || "$new_activity" != "$expected_activity" ]]; then
		echo "启动Activity不一致: old=$old_activity new=$new_activity expected=$expected_activity" >&2
		exit 4
	fi
	if [[ "$old_apk_version_code" != "$old_version_code" || "$new_apk_version_code" != "$new_version_code" ]]; then
		echo "APK版本号不一致: old=$old_apk_version_code/$old_version_code new=$new_apk_version_code/$new_version_code" >&2
		exit 4
	fi
else
	echo "未找到aapt，跳过APK静态元数据复核；安装后仍会通过PackageManager校验"
fi

if [[ -n "$apksigner_bin" ]]; then
	old_certificate=$(apk_certificate "$old_apk")
	new_certificate=$(apk_certificate "$new_apk")
	if [[ "$old_certificate" != "$expected_certificate" || "$new_certificate" != "$expected_certificate" ]]; then
		echo "签名证书不一致: old=$old_certificate new=$new_certificate expected=$expected_certificate" >&2
		exit 4
	fi
else
	echo "未找到apksigner，跳过证书静态复核；覆盖安装仍会由Android强制校验签名一致"
fi

launch_and_verify() {
  local phase=$1
  local launch_output

  "$adb_bin" shell am force-stop "$expected_package"
  launch_output=$("$adb_bin" shell am start -W -n "$expected_package/$expected_activity" 2>&1)
  printf '%s\n' "$launch_output"
  if ! grep -q 'Status: ok' <<<"$launch_output"; then
    echo "$phase 首次启动失败" >&2
    exit 5
  fi
  if ! "$adb_bin" shell pidof "$expected_package" | grep -q '[0-9]'; then
    echo "$phase 启动后未发现应用进程" >&2
    exit 5
  fi
}

echo "清理设备上的验收包（仅限 ${expected_package}）"
"$adb_bin" uninstall "$expected_package" >/dev/null 2>&1 || true

echo "安装旧版 versionCode=$old_version_code"
"$adb_bin" install "$old_apk"
launch_and_verify "旧版"
first_install_time=$("$adb_bin" shell dumpsys package "$expected_package" | sed -n 's/^[[:space:]]*firstInstallTime=//p' | head -1 | tr -d '\r')
installed_old_version_code=$("$adb_bin" shell dumpsys package "$expected_package" | sed -n 's/.*versionCode=\([0-9]*\).*/\1/p' | head -1 | tr -d '\r')
if [[ "$installed_old_version_code" != "$old_version_code" ]]; then
	echo "设备旧版版本不一致: installed=$installed_old_version_code expected=$old_version_code" >&2
	exit 5
fi

echo "覆盖安装新版 versionCode=$new_version_code"
"$adb_bin" install -r "$new_apk"
launch_and_verify "新版"
upgraded_first_install_time=$("$adb_bin" shell dumpsys package "$expected_package" | sed -n 's/^[[:space:]]*firstInstallTime=//p' | head -1 | tr -d '\r')
installed_version_code=$("$adb_bin" shell dumpsys package "$expected_package" | sed -n 's/.*versionCode=\([0-9]*\).*/\1/p' | head -1 | tr -d '\r')

if [[ -z "$first_install_time" || "$upgraded_first_install_time" != "$first_install_time" ]]; then
  echo "覆盖安装未保留首次安装时间: old=$first_install_time new=$upgraded_first_install_time" >&2
  exit 5
fi
if [[ "$installed_version_code" != "$new_version_code" ]]; then
  echo "设备安装版本不一致: installed=$installed_version_code expected=$new_version_code" >&2
  exit 5
fi

echo "V3 Android运行时验收通过"
echo "package=$expected_package"
echo "activity=$expected_activity"
echo "versionCode=$old_version_code->$new_version_code"
echo "certificateSha256=$expected_certificate"
echo "firstInstallTime=$first_install_time"
