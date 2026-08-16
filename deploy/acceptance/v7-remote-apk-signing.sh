#!/usr/bin/env bash

set -euo pipefail

# 仅使用合成 APK、测试 Keystore 和临时 mTLS 证书；不连接真实 HSM 或客户系统。
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
base_port=${APPFORGE_REMOTE_SIGNER_ACCEPTANCE_PORT:-19543}
primary_port=$base_port
tamper_port=$((base_port + 1))
delay_port=$((base_port + 2))
suffix="$(date +%s)-$$"
temporary=$(mktemp -d)
network="appforge-remote-signer-$suffix"
primary_container="appforge-remote-signer-primary-$suffix"
tamper_container="appforge-remote-signer-tamper-$suffix"
delay_container="appforge-remote-signer-delay-$suffix"
primary_replay="appforge-remote-signer-replay-$suffix"
tamper_replay="appforge-remote-signer-tamper-replay-$suffix"
delay_replay="appforge-remote-signer-delay-replay-$suffix"
evidence=${APPFORGE_REMOTE_SIGNER_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v7-remote-apk-signing-20260815.json}

cleanup() {
  docker rm -f "$primary_container" "$tamper_container" "$delay_container" >/dev/null 2>&1 || true
  docker volume rm "$primary_replay" "$tamper_replay" "$delay_replay" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT

for command in docker go openssl python3 curl shasum; do
  command -v "$command" >/dev/null || { echo "缺少命令: $command" >&2; exit 1; }
done
[[ $base_port =~ ^[1-9][0-9]*$ && $delay_port -le 65535 ]] || { echo '验收端口无效' >&2; exit 1; }
docker image inspect "$agent_image" >/dev/null 2>&1 || {
  docker build -f "$repo_root/deploy/docker/local-agent.Dockerfile" -t "$agent_image" "$repo_root" >/dev/null
}

mkdir -p "$temporary/bin" "$temporary/pki" "$temporary/server" "$temporary/client" "$temporary/wrong-client" "$temporary/fixture" "$temporary/result"
chmod 0700 "$temporary" "$temporary/pki" "$temporary/server" "$temporary/client" "$temporary/wrong-client" "$temporary/fixture" "$temporary/result"

image_arch=$(docker image inspect --format '{{.Architecture}}' "$agent_image")
case "$image_arch" in
  amd64|arm64) ;;
  *) echo "不支持的 Local Agent 镜像架构: $image_arch" >&2; exit 1 ;;
esac
(
  cd "$repo_root"
  GOCACHE=/private/tmp/appforge-go-build-cache GOOS=linux GOARCH="$image_arch" CGO_ENABLED=0 \
    go build -trimpath -o "$temporary/bin/remote-apk-signer" ./deploy/acceptance/fixtures/remote-apk-signer.go
)
(
  cd "$repo_root/common"
  GOCACHE=/private/tmp/appforge-go-build-cache go build -trimpath \
    -o "$temporary/bin/remote-apk-client" ../deploy/acceptance/fixtures/remote-apk-client.go
)
chmod 0755 "$temporary/bin/remote-apk-signer" "$temporary/bin/remote-apk-client"

create_ca() {
  local directory=$1 common_name=$2
  openssl ecparam -name prime256v1 -genkey -noout -out "$directory/ca.key"
  openssl req -x509 -new -key "$directory/ca.key" -days 1 -subj "/CN=$common_name" \
    -addext 'basicConstraints=critical,CA:TRUE' \
    -addext 'keyUsage=critical,keyCertSign,cRLSign' -out "$directory/ca.crt" >/dev/null 2>&1
}

issue_certificate() {
  local ca_directory=$1 output_directory=$2 name=$3 usage=$4 san=${5:-}
  openssl ecparam -name prime256v1 -genkey -noout -out "$output_directory/$name.key"
  openssl req -new -key "$output_directory/$name.key" -subj "/CN=$name" -out "$output_directory/$name.csr" >/dev/null 2>&1
  {
    echo 'basicConstraints=critical,CA:FALSE'
    echo 'keyUsage=critical,digitalSignature'
    echo "extendedKeyUsage=$usage"
    [[ -z $san ]] || echo "subjectAltName=$san"
  } >"$output_directory/$name.ext"
  openssl x509 -req -in "$output_directory/$name.csr" -CA "$ca_directory/ca.crt" -CAkey "$ca_directory/ca.key" \
    -CAcreateserial -days 1 -extfile "$output_directory/$name.ext" -out "$output_directory/$name.crt" >/dev/null 2>&1
}

create_ca "$temporary/pki" 'AppForge Remote APK Signer Acceptance CA'
issue_certificate "$temporary/pki" "$temporary/server" server serverAuth 'DNS:localhost,IP:127.0.0.1'
cp "$temporary/pki/ca.crt" "$temporary/server/ca.crt"
cp "$temporary/pki/ca.crt" "$temporary/client/ca.crt"
issue_certificate "$temporary/pki" "$temporary/client" builder clientAuth
create_ca "$temporary/wrong-client" 'AppForge Untrusted Acceptance CA'
issue_certificate "$temporary/wrong-client" "$temporary/wrong-client" untrusted-builder clientAuth
cp "$temporary/server/ca.crt" "$temporary/wrong-client/server-ca.crt"
chmod 0600 "$temporary"/{pki,server,client,wrong-client}/*.key
chmod 0644 "$temporary"/{server,client,wrong-client}/*.crt

docker run --rm --user 0 -v "$temporary/fixture:/out" --entrypoint sh "$agent_image" -lc '
set -eu
cat >/tmp/AndroidManifest.xml <<"EOF"
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.appforge.remotesigner.acceptance" android:versionCode="1" android:versionName="1.0">
  <uses-sdk android:minSdkVersion="21" />
  <application android:allowBackup="false" android:label="Remote Signer Acceptance" />
</manifest>
EOF
aapt package -f -M /tmp/AndroidManifest.xml -I /usr/share/android-framework-res/framework-res.apk -F /out/source.apk
zipalign -f 4 /out/source.apk /out/aligned.apk
keytool -genkeypair -noprompt -alias release -keyalg RSA -keysize 2048 -validity 30 \
  -dname "CN=AppForge Remote Signer Acceptance,O=AppForge,C=CN" \
  -keystore /out/release.jks -storepass changeit -keypass changeit >/dev/null 2>&1
keytool -exportcert -alias release -keystore /out/release.jks -storepass changeit -file /out/release.der >/dev/null 2>&1
chmod 0600 /out/source.apk /out/aligned.apk /out/release.jks /out/release.der
'
certificate_sha=$(shasum -a 256 "$temporary/fixture/release.der" | awk '{print $1}')

write_secret() {
  local endpoint=$1 key_id=$2 ca=$3 cert=$4 key=$5 output=$6
  python3 - "$endpoint" "$key_id" "$ca" "$cert" "$key" "$output" <<'PY'
import json, pathlib, sys
endpoint, key_id, ca, cert, key, output = sys.argv[1:]
payload = {
    "endpoint": endpoint,
    "keyId": key_id,
    "caCertificatePem": pathlib.Path(ca).read_text(),
    "clientCertificatePem": pathlib.Path(cert).read_text(),
    "clientPrivateKeyPem": pathlib.Path(key).read_text(),
    "serverName": "localhost",
}
pathlib.Path(output).write_text(json.dumps(payload, separators=(",", ":")))
PY
  chmod 0600 "$output"
}

write_secret "https://127.0.0.1:$primary_port" android-release "$temporary/client/ca.crt" \
  "$temporary/client/builder.crt" "$temporary/client/builder.key" "$temporary/client/primary.json"
write_secret "https://127.0.0.1:$primary_port" wrong-key "$temporary/client/ca.crt" \
  "$temporary/client/builder.crt" "$temporary/client/builder.key" "$temporary/client/wrong-key.json"
write_secret "https://127.0.0.1:$primary_port" android-release "$temporary/wrong-client/server-ca.crt" \
  "$temporary/wrong-client/untrusted-builder.crt" "$temporary/wrong-client/untrusted-builder.key" "$temporary/wrong-client/wrong-client.json"
write_secret "https://127.0.0.1:$tamper_port" android-release "$temporary/client/ca.crt" \
  "$temporary/client/builder.crt" "$temporary/client/builder.key" "$temporary/client/tamper.json"
write_secret "https://127.0.0.1:$delay_port" android-release "$temporary/client/ca.crt" \
  "$temporary/client/builder.crt" "$temporary/client/builder.key" "$temporary/client/delay.json"

# 独立 bridge 仅承载临时签名夹具；宿主机需要通过发布端口完成 mTLS 验收。
docker network create "$network" >/dev/null
for volume in "$primary_replay" "$tamper_replay" "$delay_replay"; do docker volume create "$volume" >/dev/null; done

start_signer() {
  local name=$1 port=$2 replay=$3
  shift 3
  docker run -d --name "$name" --user 0 --network "$network" -p "127.0.0.1:$port:9443" \
    -e APPFORGE_TEST_KEYSTORE_PASSWORD=changeit -e APPFORGE_TEST_KEY_PASSWORD=changeit \
    -v "$temporary/bin/remote-apk-signer:/usr/local/bin/remote-apk-signer:ro" \
    -v "$temporary/server:/etc/appforge-remote-signer:ro" \
    -v "$temporary/fixture/release.jks:/fixture/release.jks:ro" -v "$replay:/replay" \
    --entrypoint /usr/local/bin/remote-apk-signer "$agent_image" \
    --listen :9443 --tls-cert /etc/appforge-remote-signer/server.crt \
    --tls-key /etc/appforge-remote-signer/server.key --client-ca /etc/appforge-remote-signer/ca.crt \
    --keystore /fixture/release.jks --key-alias release --key-id android-release --replay-dir /replay "$@" >/dev/null
}

wait_signer() {
  local secret=$1 container=$2
  for _ in $(seq 1 60); do
    if "$temporary/bin/remote-apk-client" -secret "$secret" -info-only -timeout 1s >/dev/null 2>&1; then return; fi
    sleep 1
  done
  docker logs "$container" >&2 || true
  echo "远程签名测试服务未就绪: $container" >&2
  exit 1
}

start_signer "$primary_container" "$primary_port" "$primary_replay"
wait_signer "$temporary/client/primary.json" "$primary_container"

nonce=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n')
"$temporary/bin/remote-apk-client" -secret "$temporary/client/primary.json" \
  -input "$temporary/fixture/aligned.apk" -output "$temporary/result/signed.apk" \
  -task 7001 -attempt 1 -certificate-sha256 "$certificate_sha" -nonce "$nonce" >"$temporary/result/success.txt"
signed_mode=$(python3 -c 'import os,stat,sys; print(oct(stat.S_IMODE(os.stat(sys.argv[1]).st_mode))[2:])' "$temporary/result/signed.apk")
[[ $signed_mode == 600 ]] || { echo '签名 APK 权限不是 0600' >&2; exit 1; }

actual_certificate=$(docker run --rm --user 0 -v "$temporary/result:/result:ro" --entrypoint sh "$agent_image" -lc \
  "apksigner verify --verbose --print-certs /result/signed.apk >/tmp/verify.txt && zipalign -c 4 /result/signed.apk >/dev/null && aapt dump badging /result/signed.apk | grep -q \"package: name='com.appforge.remotesigner.acceptance'\" && sed -n 's/^Signer #1 certificate SHA-256 digest: //p' /tmp/verify.txt | tr '[:upper:]' '[:lower:]' | head -1")
[[ $actual_certificate == "$certificate_sha" ]] || { echo '最终 APK 签名证书不匹配' >&2; exit 1; }

if "$temporary/bin/remote-apk-client" -secret "$temporary/client/primary.json" \
  -input "$temporary/fixture/aligned.apk" -output "$temporary/result/replayed.apk" \
  -task 7001 -attempt 1 -certificate-sha256 "$certificate_sha" -nonce "$nonce" 2>"$temporary/result/replay.err"; then
  echo '相同 nonce 被重复签名' >&2
  exit 1
fi
grep -q 'HTTP 409' "$temporary/result/replay.err"

docker rm -f "$primary_container" >/dev/null
start_signer "$primary_container" "$primary_port" "$primary_replay"
wait_signer "$temporary/client/primary.json" "$primary_container"
if "$temporary/bin/remote-apk-client" -secret "$temporary/client/primary.json" \
  -input "$temporary/fixture/aligned.apk" -output "$temporary/result/replayed-after-restart.apk" \
  -task 7001 -attempt 1 -certificate-sha256 "$certificate_sha" -nonce "$nonce" 2>"$temporary/result/replay-after-restart.err"; then
  echo '签名服务重启后防重放状态丢失' >&2
  exit 1
fi
grep -q 'HTTP 409' "$temporary/result/replay-after-restart.err"

request_timestamp=$(python3 -c 'from datetime import datetime,timezone; print(datetime.now(timezone.utc).isoformat().replace("+00:00","Z"))')
request_nonce=$(openssl rand -base64 32 | tr '+/' '-_' | tr -d '=\n')
tampered_request_code=$(curl -sS -o "$temporary/result/tampered-request.txt" -w '%{http_code}' \
  --cacert "$temporary/client/ca.crt" --cert "$temporary/client/builder.crt" --key "$temporary/client/builder.key" \
  -H 'Content-Type: application/vnd.android.package-archive' -H 'X-AppForge-Schema-Version: 1' \
  -H 'X-AppForge-Task-Id: 7002' -H 'X-AppForge-Builder-Attempt: 1' -H 'X-AppForge-Key-Id: android-release' \
  -H "X-AppForge-Request-Nonce: $request_nonce" -H "X-AppForge-Request-Timestamp: $request_timestamp" \
  -H "X-AppForge-Unsigned-Sha256: $(printf '0%.0s' {1..64})" --data-binary @"$temporary/fixture/aligned.apk" \
  "https://127.0.0.1:$primary_port/v1/sign-apk")
[[ $tampered_request_code == 400 ]] || { echo "篡改请求未被拒绝: HTTP $tampered_request_code" >&2; exit 1; }

if "$temporary/bin/remote-apk-client" -secret "$temporary/wrong-client/wrong-client.json" -info-only -timeout 5s >/dev/null 2>&1; then
  echo '未受信客户端证书被接受' >&2
  exit 1
fi
if "$temporary/bin/remote-apk-client" -secret "$temporary/client/wrong-key.json" -info-only -timeout 5s >/dev/null 2>&1; then
  echo '错误 keyId 被接受' >&2
  exit 1
fi
if "$temporary/bin/remote-apk-client" -secret "$temporary/client/primary.json" \
  -input "$temporary/fixture/aligned.apk" -output "$temporary/result/wrong-certificate.apk" \
  -task 7003 -attempt 1 -certificate-sha256 "$(printf 'b%.0s' {1..64})" >/dev/null 2>&1; then
  echo '错误签名证书指纹被接受' >&2
  exit 1
fi
[[ ! -e "$temporary/result/wrong-certificate.apk" ]]

start_signer "$tamper_container" "$tamper_port" "$tamper_replay" --tamper-response
wait_signer "$temporary/client/tamper.json" "$tamper_container"
if "$temporary/bin/remote-apk-client" -secret "$temporary/client/tamper.json" \
  -input "$temporary/fixture/aligned.apk" -output "$temporary/result/tampered-response.apk" \
  -task 7004 -attempt 1 -certificate-sha256 "$certificate_sha" 2>"$temporary/result/tampered-response.err"; then
  echo '篡改签名响应被接受' >&2
  exit 1
fi
grep -q 'SHA-256 mismatch' "$temporary/result/tampered-response.err"
[[ ! -e "$temporary/result/tampered-response.apk" ]]

start_signer "$delay_container" "$delay_port" "$delay_replay" --response-delay 3s
wait_signer "$temporary/client/delay.json" "$delay_container"
if "$temporary/bin/remote-apk-client" -secret "$temporary/client/delay.json" \
  -input "$temporary/fixture/aligned.apk" -output "$temporary/result/timeout.apk" \
  -task 7005 -attempt 1 -certificate-sha256 "$certificate_sha" -timeout 1s 2>"$temporary/result/timeout.err"; then
  echo '超时签名请求未失败' >&2
  exit 1
fi
grep -Eq 'deadline exceeded|Client.Timeout|context deadline' "$temporary/result/timeout.err"
[[ ! -e "$temporary/result/timeout.apk" ]]

for container in "$primary_container" "$tamper_container" "$delay_container"; do
  if docker logs "$container" 2>&1 | grep -Eq 'changeit|BEGIN (EC |RSA )?PRIVATE KEY|clientPrivateKeyPem'; then
    echo "签名服务日志泄露测试凭据: $container" >&2
    exit 1
  fi
done

mkdir -p "$(dirname "$evidence")"
EVIDENCE_PATH="$evidence" CERTIFICATE_SHA="$certificate_sha" python3 - <<'PY'
import json, os, pathlib
path = pathlib.Path(os.environ["EVIDENCE_PATH"])
payload = {
    "schemaVersion": 1,
    "date": "2026-08-15",
    "mode": "synthetic-remote-apk-signer",
    "realHSM": False,
    "productionCredentialsUsed": False,
    "certificateSha256": os.environ["CERTIFICATE_SHA"],
    "checks": {
        "mutualTLS": "passed",
        "validAPKSignature": "passed",
        "certificateBinding": "passed",
        "requestDigestTamperRejected": "passed",
        "responseDigestTamperRejected": "passed",
        "untrustedClientRejected": "passed",
        "wrongKeyIdRejected": "passed",
        "wrongCertificateRejected": "passed",
        "persistentReplayProtection": "passed",
        "timeoutRejectedWithoutOutput": "passed",
        "secretLogLeakScan": "passed",
    },
    "limitations": [
        "The signer is a local apksigner-backed protocol fixture, not a real HSM.",
        "Persistent Core/API/Builder task integration is validated separately by v7-remote-signing-task-e2e.sh.",
    ],
}
path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n")
PY
chmod 0600 "$evidence"

echo "REMOTE_APK_SIGNER 合成 E2E 通过：mTLS、APK签名、请求/响应防篡改、持久防重放、证书绑定与超时均已验证。"
echo "证据: $evidence"
