#!/usr/bin/env sh
set -eu
set +x

: "${APPFORGE_BASE_URL:?APPFORGE_BASE_URL is required}"
: "${APPFORGE_API_KEY:?APPFORGE_API_KEY is required}"
: "${APPFORGE_APP_ID:?APPFORGE_APP_ID is required}"
: "${APPFORGE_CHANNEL_ID:?APPFORGE_CHANNEL_ID is required}"
: "${APPFORGE_SIGNING_CONFIG_ID:?APPFORGE_SIGNING_CONFIG_ID is required}"
: "${APPFORGE_APK:?APPFORGE_APK is required}"
: "${APPFORGE_VERSION_CODE:?APPFORGE_VERSION_CODE is required}"
: "${APPFORGE_VERSION_NAME:?APPFORGE_VERSION_NAME is required}"

APPFORGE_CLI="${APPFORGE_CLI:-appforgectl}"
APPFORGE_OUTPUT="${APPFORGE_OUTPUT:-channel.apk}"

version_json="$("${APPFORGE_CLI}" --json version upload \
  --app-id "${APPFORGE_APP_ID}" \
  --file "${APPFORGE_APK}" \
  --version-code "${APPFORGE_VERSION_CODE}" \
  --version-name "${APPFORGE_VERSION_NAME}")"
version_id="$(printf '%s' "${version_json}" | jq -er '.id')"

build_json="$("${APPFORGE_CLI}" --json build create \
  --app-id "${APPFORGE_APP_ID}" \
  --version-id "${version_id}" \
  --channel-id "${APPFORGE_CHANNEL_ID}" \
  --signing-config-id "${APPFORGE_SIGNING_CONFIG_ID}")"
build_id="$(printf '%s' "${build_json}" | jq -er '.id')"

result_json="$("${APPFORGE_CLI}" --json --timeout "${APPFORGE_BUILD_TIMEOUT:-30m}" build wait --id "${build_id}")"
artifact_id="$(printf '%s' "${result_json}" | jq -er '.apkObjectId')"

"${APPFORGE_CLI}" --json artifact download --id "${artifact_id}" --output "${APPFORGE_OUTPUT}" >/dev/null
printf 'AppForge build %s completed; artifact saved to %s\n' "${build_id}" "${APPFORGE_OUTPUT}"
