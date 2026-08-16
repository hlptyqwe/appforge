#!/usr/bin/env bash

set -euo pipefail

# 只使用合成 APK、测试 Keystore、临时 mTLS 证书和唯一临时 Compose 栈。
# 不连接真实 HSM、客户对象存储、共享开发数据库或生产凭据。
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
temporary=$(mktemp -d)
project="appforge-v7-remote-task-$(date +%s)-$$"
compose_file="$temporary/docker-compose.yml"
override_file="$temporary/docker-compose.override.yml"
etcd_dir="$temporary/etcd"
seed_file="$temporary/30-seed.sql"
secret_dir="$temporary/secrets"
evidence=${APPFORGE_REMOTE_SIGNER_TASK_EVIDENCE_PATH:-$repo_root/docs/enterprise/evidence/v7-remote-signing-task-20260815.json}
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
stack_started=false

[[ $project =~ ^appforge-v7-remote-task-[0-9]+-[0-9]+$ ]]

compose() {
  docker compose -p "$project" --project-directory "$repo_root/deploy" \
    -f "$compose_file" -f "$override_file" "$@"
}

cleanup() {
  local exit_code=$?
  set +e
  if [[ $stack_started == true ]]; then
    if ((exit_code != 0)); then
      echo '远程签名任务 E2E 失败，输出隔离栈状态与关键日志后销毁：' >&2
      compose ps -a >&2
      compose logs --no-color mysql-migrate etcd-init remote-signer core-rpc builder-rpc builder-worker-no-remote builder-worker-1 admin-api >&2
    fi
    compose down -v --remove-orphans >/dev/null 2>&1
  fi
  [[ $temporary == /tmp/* || $temporary == /private/var/folders/* || $temporary == /var/folders/* ]] && rm -rf "$temporary"
}
trap cleanup EXIT
trap 'echo "远程签名任务 E2E 在第 ${LINENO} 行失败" >&2' ERR

for command in awk basename cat cp curl date docker go grep mv openssl python3 sed seq shasum sleep tr wc; do
  command -v "$command" >/dev/null || { echo "缺少命令: $command" >&2; exit 1; }
done

required_images=(
  appforge-dev-etcd-init:latest appforge-dev-system-rpc:latest appforge-dev-core-rpc:latest
  appforge-dev-builder-rpc:latest appforge-dev-builder-worker-1:latest appforge-dev-admin-api:latest
  "$agent_image" mysql:8.4 redis:7.4-alpine quay.io/coreos/etcd:v3.6.12
  minio/minio:RELEASE.2025-04-22T22-12-26Z minio/mc:RELEASE.2025-04-16T18-13-26Z
)
for image in "${required_images[@]}"; do
  docker image inspect "$image" >/dev/null || { echo "缺少本地镜像: $image" >&2; exit 1; }
done

mkdir -p "$etcd_dir" "$secret_dir" "$temporary/bin" "$temporary/pki" "$temporary/server" \
  "$temporary/client" "$temporary/fixture" "$temporary/result"
chmod 0700 "$temporary" "$etcd_dir" "$secret_dir" "$temporary/pki" "$temporary/server" \
  "$temporary/client" "$temporary/fixture" "$temporary/result"
cp "$repo_root/deploy/etcd/"*.yaml "$etcd_dir/"
cp "$repo_root/deploy/etcd/init-etcd.sh" "$etcd_dir/init-etcd.sh"

read -r mysql_port redis_port etcd_port minio_port minio_console_port system_port core_port builder_port api_port <<<"$(python3 - <<'PY'
import socket
sockets=[]
try:
    for _ in range(9):
        sock=socket.socket(); sock.bind(('127.0.0.1',0)); sockets.append(sock)
    print(*(sock.getsockname()[1] for sock in sockets))
finally:
    for sock in sockets: sock.close()
PY
)"

sed "s#BucketUrl: http://localhost:9000/appforge#BucketUrl: http://127.0.0.1:${minio_port}/appforge#" \
  "$etcd_dir/builder.yaml" >"$etcd_dir/builder.yaml.tmp"
mv "$etcd_dir/builder.yaml.tmp" "$etcd_dir/builder.yaml"
sed 's#  LocalRoot: ""#  LocalRoot: /run/appforge-remote-secrets#' "$etcd_dir/builder.yaml" >"$etcd_dir/builder.yaml.tmp"
mv "$etcd_dir/builder.yaml.tmp" "$etcd_dir/builder.yaml"
sed 's#"cache":true}#"cache":true,"remoteSigning":true}#' "$etcd_dir/builder.yaml" >"$etcd_dir/builder.yaml.tmp"
mv "$etcd_dir/builder.yaml.tmp" "$etcd_dir/builder.yaml"
grep -F 'LocalRoot: /run/appforge-remote-secrets' "$etcd_dir/builder.yaml" >/dev/null
grep -F '"remoteSigning":true' "$etcd_dir/builder.yaml" >/dev/null

create_ca() {
  openssl ecparam -name prime256v1 -genkey -noout -out "$temporary/pki/ca.key"
  openssl req -x509 -new -key "$temporary/pki/ca.key" -days 1 \
    -subj '/CN=AppForge Remote Task E2E CA' -addext 'basicConstraints=critical,CA:TRUE' \
    -addext 'keyUsage=critical,keyCertSign,cRLSign' -out "$temporary/pki/ca.crt" >/dev/null 2>&1
}

issue_certificate() {
  local directory=$1 name=$2 usage=$3 san=${4:-}
  openssl ecparam -name prime256v1 -genkey -noout -out "$directory/$name.key"
  openssl req -new -key "$directory/$name.key" -subj "/CN=$name" -out "$directory/$name.csr" >/dev/null 2>&1
  {
    echo 'basicConstraints=critical,CA:FALSE'
    echo 'keyUsage=critical,digitalSignature'
    echo "extendedKeyUsage=$usage"
    [[ -z $san ]] || echo "subjectAltName=$san"
  } >"$directory/$name.ext"
  openssl x509 -req -in "$directory/$name.csr" -CA "$temporary/pki/ca.crt" -CAkey "$temporary/pki/ca.key" \
    -CAcreateserial -days 1 -extfile "$directory/$name.ext" -out "$directory/$name.crt" >/dev/null 2>&1
}

create_ca
issue_certificate "$temporary/server" server serverAuth 'DNS:remote-signer'
issue_certificate "$temporary/client" builder clientAuth
cp "$temporary/pki/ca.crt" "$temporary/server/ca.crt"
cp "$temporary/pki/ca.crt" "$temporary/client/ca.crt"
chmod 0600 "$temporary"/{pki,server,client}/*.key

image_arch=$(docker image inspect --format '{{.Architecture}}' "$agent_image")
[[ $image_arch == amd64 || $image_arch == arm64 ]]
(
  cd "$repo_root"
  GOCACHE=/private/tmp/appforge-go-build-cache GOOS=linux GOARCH="$image_arch" CGO_ENABLED=0 \
    go build -trimpath -o "$temporary/bin/remote-apk-signer" ./deploy/acceptance/fixtures/remote-apk-signer.go
)
chmod 0755 "$temporary/bin/remote-apk-signer"

package_name="com.appforge.remote.task.n$(date +%s)"
docker run --rm --user 0 -e PACKAGE_NAME="$package_name" -v "$temporary/fixture:/out" \
  --entrypoint sh "$agent_image" -lc '
set -eu
cat >/tmp/AndroidManifest.xml <<EOF
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="$PACKAGE_NAME" android:versionCode="1" android:versionName="1.0">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="35" />
  <application android:allowBackup="false" android:label="Remote Task E2E" />
</manifest>
EOF
aapt package -f -M /tmp/AndroidManifest.xml -I /usr/share/android-framework-res/framework-res.apk -F /out/source.apk
keytool -genkeypair -noprompt -alias release -keyalg RSA -keysize 2048 -validity 30 \
  -dname "CN=AppForge Remote Task E2E,O=AppForge,C=CN" -keystore /out/release.jks \
  -storepass changeit -keypass changeit >/dev/null 2>&1
keytool -exportcert -alias release -keystore /out/release.jks -storepass changeit -file /out/release.der >/dev/null 2>&1
chmod 0600 /out/source.apk /out/release.jks /out/release.der
'
certificate_sha=$(shasum -a 256 "$temporary/fixture/release.der" | awk '{print $1}')

CA_FILE="$temporary/client/ca.crt" CERT_FILE="$temporary/client/builder.crt" \
KEY_FILE="$temporary/client/builder.key" OUTPUT_FILE="$secret_dir/remote.json" python3 - <<'PY'
import json,os,pathlib
payload={
  'endpoint':'https://remote-signer:9443',
  'keyId':'android-release',
  'caCertificatePem':pathlib.Path(os.environ['CA_FILE']).read_text(),
  'clientCertificatePem':pathlib.Path(os.environ['CERT_FILE']).read_text(),
  'clientPrivateKeyPem':pathlib.Path(os.environ['KEY_FILE']).read_text(),
  'serverName':'remote-signer',
}
pathlib.Path(os.environ['OUTPUT_FILE']).write_text(json.dumps(payload,separators=(',',':')))
PY
chmod 0600 "$secret_dir/remote.json"

sed "s#http://localhost:9000/appforge#http://127.0.0.1:${minio_port}/appforge#g" \
  "$repo_root/deploy/mysql/init/30-seed.sql" >"$seed_file"
! grep -F 'http://localhost:9000/appforge' "$seed_file" >/dev/null

awk '
  NR == 1 && $0 == "name: appforge-dev" { next }
  /^[[:space:]]+container_name:/ { next }
  $0 == "    name: appforge-dev" { next }
  { print }
' "$repo_root/deploy/docker-compose.dev.yml" >"$compose_file"

cat >"$override_file" <<EOF
services:
  mysql:
    ports: !reset []
    volumes:
      - mysql-data:/var/lib/mysql
      - $repo_root/services/system/system.sql:/docker-entrypoint-initdb.d/10-system.sql:ro
      - $repo_root/services/core/core.sql:/docker-entrypoint-initdb.d/20-core.sql:ro
      - $seed_file:/docker-entrypoint-initdb.d/30-seed.sql:ro
  mysql-migrate:
    volumes:
      - $repo_root/deploy/mysql/migrations:/migrations:ro
  redis:
    ports: !reset []
  etcd:
    ports: !reset []
  minio:
    ports: !override
      - "127.0.0.1:$minio_port:9000"
  etcd-init:
    image: appforge-dev-etcd-init:latest
    volumes:
      - $etcd_dir:/config:ro
      - $etcd_dir/init-etcd.sh:/init/init-etcd.sh:ro
  system-rpc:
    image: appforge-dev-system-rpc:latest
    ports: !reset []
  core-rpc:
    image: appforge-dev-core-rpc:latest
    ports: !reset []
  builder-rpc:
    image: appforge-dev-builder-rpc:latest
    ports: !reset []
    volumes:
      - $secret_dir:/run/appforge-remote-secrets:ro
    depends_on:
      remote-signer:
        condition: service_started
  builder-worker-1:
    image: appforge-dev-builder-worker-1:latest
    volumes:
      - $secret_dir:/run/appforge-remote-secrets:ro
    depends_on:
      remote-signer:
        condition: service_started
  builder-worker-no-remote:
    image: appforge-dev-builder-worker-1:latest
    restart: unless-stopped
    command: ["-etcd", "etcd:2379"]
    environment:
      APPFORGE_BUILDER_ID: builder-no-remote
      APPFORGE_BUILDER_ENDPOINT: builder-worker-no-remote
      APPFORGE_BUILDER_CAPABILITY_JSON: '{"apk":true,"branding":true,"whiteLabel":true,"cache":true}'
    volumes:
      - $secret_dir:/run/appforge-remote-secrets:ro
    depends_on:
      remote-signer:
        condition: service_started
      builder-rpc:
        condition: service_healthy
      core-rpc:
        condition: service_healthy
    networks:
      - appforge
  admin-api:
    image: appforge-dev-admin-api:latest
    ports: !override
      - "127.0.0.1:$api_port:8888"
  remote-signer:
    image: $agent_image
    user: "0"
    entrypoint: ["/usr/local/bin/remote-apk-signer"]
    command: ["--listen", ":9443", "--tls-cert", "/signer/server.crt", "--tls-key", "/signer/server.key", "--client-ca", "/signer/ca.crt", "--keystore", "/fixture/release.jks", "--key-alias", "release", "--key-id", "android-release", "--replay-dir", "/replay"]
    environment:
      APPFORGE_TEST_KEYSTORE_PASSWORD: changeit
      APPFORGE_TEST_KEY_PASSWORD: changeit
    volumes:
      - $temporary/bin/remote-apk-signer:/usr/local/bin/remote-apk-signer:ro
      - $temporary/server:/signer:ro
      - $temporary/fixture/release.jks:/fixture/release.jks:ro
      - remote-replay:/replay
    networks:
      - appforge
volumes:
  remote-replay:
EOF

export APPFORGE_MYSQL_PORT=$mysql_port APPFORGE_REDIS_PORT=$redis_port APPFORGE_ETCD_PORT=$etcd_port
export APPFORGE_MINIO_PORT=$minio_port APPFORGE_MINIO_CONSOLE_PORT=$minio_console_port
export APPFORGE_SYSTEM_RPC_PORT=$system_port APPFORGE_CORE_RPC_PORT=$core_port
export APPFORGE_BUILDER_RPC_PORT=$builder_port APPFORGE_API_PORT=$api_port

rendered=$(compose config)
! grep -F 'container_name:' <<<"$rendered" >/dev/null
grep -F "name: ${project}_appforge" <<<"$rendered" >/dev/null
grep -F 'target: /run/appforge-remote-secrets' <<<"$rendered" >/dev/null

stack_started=true
compose up -d --no-build mysql mysql-migrate redis etcd etcd-init minio minio-init remote-signer \
  system-rpc core-rpc builder-rpc builder-worker-no-remote admin-api

for _ in $(seq 1 120); do
  curl -fsS "http://127.0.0.1:${api_port}/admin/system/core" >/dev/null 2>&1 && break
  sleep 1
done
curl -fsS "http://127.0.0.1:${api_port}/admin/system/core" >/dev/null

responses="$temporary/responses"
mkdir -p "$responses"
login=$(curl -fsS -H 'Content-Type: application/json' -d '{"username":"agent","password":"AppForge@123"}' \
  "http://127.0.0.1:${api_port}/agent/auth/login")
token=$(printf '%s' "$login" | python3 -c 'import json,sys;p=json.load(sys.stdin);assert p["code"]==200;print(p["data"]["token"])')
auth=(-H "Authorization: Bearer $token")

api_post() {
  local path=$1 body=$2 output=$3
  curl -fsS "${auth[@]}" -H 'Content-Type: application/json' -d "$body" \
    "http://127.0.0.1:${api_port}/agent/core$path" >"$output"
  RESPONSE_FILE="$output" python3 -c 'import json,os;p=json.load(open(os.environ["RESPONSE_FILE"]));assert p["code"]==200,p'
}

json_id() { RESPONSE_FILE=$1 python3 -c 'import json,os;print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["id"])'; }

api_post /applications "{\"appCode\":\"remote-task-$(date +%s)\",\"appName\":\"远程签名合成任务\",\"packageName\":\"$package_name\",\"apiHost\":\"https://remote-task.invalid\"}" "$responses/app.json"
app_id=$(json_id "$responses/app.json")
tenant_id=$(RESPONSE_FILE="$responses/app.json" python3 -c 'import json,os;print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["tenantId"])')

size=$(wc -c <"$temporary/fixture/source.apk" | tr -d ' ')
api_post /uploads/initiate "{\"appId\":$app_id,\"objectType\":1,\"fileName\":\"source.apk\",\"sizeBytes\":$size,\"contentType\":\"application/vnd.android.package-archive\"}" "$responses/upload.json"
object_id=$(RESPONSE_FILE="$responses/upload.json" python3 -c 'import json,os;print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["objectId"])')
upload_url=$(RESPONSE_FILE="$responses/upload.json" python3 -c 'import json,os;print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["uploadUrl"])')
[[ $upload_url == "http://127.0.0.1:${minio_port}/"* ]]
curl -fsS -X PUT -H 'Content-Type: application/vnd.android.package-archive' --data-binary @"$temporary/fixture/source.apk" "$upload_url" >/dev/null
api_post "/uploads/$object_id/complete" '{}' "$responses/upload-complete.json"

api_post /versions "{\"appId\":$app_id,\"versionCode\":1,\"versionName\":\"1.0\",\"sourceApkObjectId\":$object_id}" "$responses/version.json"
version_id=$(json_id "$responses/version.json")
api_post /channels "{\"appId\":$app_id,\"channelCode\":\"remote\",\"channelName\":\"Remote Signer\"}" "$responses/channel.json"
channel_id=$(json_id "$responses/channel.json")
api_post /signing-configs "{\"appId\":$app_id,\"name\":\"remote-signer\",\"signingMode\":2,\"keyAlias\":\"android-release\",\"secretRef\":\"local-file:///remote.json\",\"keystoreObjectId\":0}" "$responses/signing.json"
signing_id=$(json_id "$responses/signing.json")

SIGNING_FILE="$responses/signing.json" CERTIFICATE_SHA="$certificate_sha" python3 - <<'PY'
import json,os
d=json.load(open(os.environ['SIGNING_FILE']))['data']
assert d['signingMode']==2 and d['keystoreObjectId']==0
assert d['keyAlias']=='android-release' and d['secretRef']=='local-file:///remote.json'
assert d['certificateSha256']==os.environ['CERTIFICATE_SHA']
PY

mysql_container="$project-mysql-1"
mysql=(docker exec -e MYSQL_PWD=appforge_dev_password "$mysql_container" mysql -uappforge -D appforge -N -B)
read -r db_keystore_id db_object_key db_secret_ref db_passwords <<<"$("${mysql[@]}" -e "SELECT keystore_object_id,IF(keystore_object_key='', 'EMPTY', keystore_object_key),secret_ref,(keystore_password_ciphertext IS NOT NULL)+(key_password_ciphertext IS NOT NULL) FROM t_app_signing_config WHERE id=$signing_id AND tenant_id=$tenant_id")"
[[ $db_keystore_id == 0 && $db_object_key == EMPTY && $db_secret_ref == local-file:///remote.json && $db_passwords == 0 ]]

api_post /build-tasks "{\"appId\":$app_id,\"versionId\":$version_id,\"channelId\":$channel_id,\"signingConfigId\":$signing_id,\"poolCode\":\"default\"}" "$responses/build.json"
task_id=$(json_id "$responses/build.json")
sleep 5
read -r gated_status gated_builder <<<"$("${mysql[@]}" -e "SELECT status,IFNULL(builder_id,'EMPTY') FROM t_build_task WHERE id=$task_id AND tenant_id=$tenant_id")"
[[ $gated_status == PENDING && $gated_builder == EMPTY ]]
non_remote_capability=$("${mysql[@]}" -e "SELECT JSON_EXTRACT(capability_json,'$.remoteSigning') FROM t_builder_node WHERE node_code='builder-no-remote'")
[[ $non_remote_capability == NULL ]]
compose up -d --no-build builder-worker-1
for _ in $(seq 1 180); do
  curl -fsS "${auth[@]}" "http://127.0.0.1:${api_port}/agent/core/build-tasks/$task_id" >"$responses/build-result.json"
  task_status=$(RESPONSE_FILE="$responses/build-result.json" python3 -c 'import json,os;print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["status"])')
  [[ $task_status == 5 || $task_status == 6 || $task_status == 7 ]] && break
  sleep 1
done
if [[ ${task_status:-0} != 5 ]]; then
  cat "$responses/build-result.json" >&2
  exit 1
fi
apk_object_id=$(RESPONSE_FILE="$responses/build-result.json" python3 -c 'import json,os;print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["apkObjectId"])')
builder_attempt=$(RESPONSE_FILE="$responses/build-result.json" python3 -c 'import json,os;print(json.load(open(os.environ["RESPONSE_FILE"]))["data"]["builderAttempt"])')
curl -fsS "${auth[@]}" "http://127.0.0.1:${api_port}/agent/core/storage/objects/$apk_object_id/download" >"$responses/download.json"
download_url=$(RESPONSE_FILE="$responses/download.json" python3 -c 'import json,os;p=json.load(open(os.environ["RESPONSE_FILE"]));assert p["code"]==200;print(p["data"]["downloadUrl"])')
[[ $download_url == "http://127.0.0.1:${minio_port}/"* ]]
curl -fsS "$download_url" -o "$temporary/result/channel.apk"

verification=$(docker run --rm --user 0 -v "$temporary/result:/result:ro" --entrypoint sh "$agent_image" -lc '
set -eu
apksigner verify --verbose --print-certs /result/channel.apk
zipalign -c 4 /result/channel.apk >/dev/null
aapt dump badging /result/channel.apk
')
grep -F "package: name='$package_name'" <<<"$verification" >/dev/null
actual_certificate=$(sed -n 's/^Signer #1 certificate SHA-256 digest: //p' <<<"$verification" | tr '[:upper:]' '[:lower:]' | head -1)
[[ $actual_certificate == "$certificate_sha" ]]
APK_FILE="$temporary/result/channel.apk" python3 - <<'PY'
import json,os,zipfile
with zipfile.ZipFile(os.environ['APK_FILE']) as archive:
    payload=json.loads(archive.read('assets/appforge/channel.json'))
assert payload['channelCode']=='remote'
PY

signer_container="$project-remote-signer-1"
! docker logs "$signer_container" 2>&1 | grep -Eq 'changeit|BEGIN (EC |RSA )?PRIVATE KEY|clientPrivateKeyPem'
read -r db_status db_attempt db_apk_id db_log_id <<<"$("${mysql[@]}" -e "SELECT status,builder_attempt,apk_object_id,log_object_id FROM t_build_task WHERE id=$task_id AND tenant_id=$tenant_id")"
[[ $db_status == SUCCESS && $db_attempt == "$builder_attempt" && $db_apk_id == "$apk_object_id" && $db_log_id -gt 0 ]]
keystore_objects=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_storage_object WHERE app_id=$app_id AND object_type=2")
[[ $keystore_objects == 0 ]]

mkdir -p "$(dirname "$evidence")"
EVIDENCE_PATH="$evidence" PROJECT="$project" TENANT_ID="$tenant_id" APP_ID="$app_id" SIGNING_ID="$signing_id" \
TASK_ID="$task_id" BUILDER_ATTEMPT="$builder_attempt" APK_OBJECT_ID="$apk_object_id" CERTIFICATE_SHA="$certificate_sha" python3 - <<'PY'
import json,os,pathlib
payload={
  'schemaVersion':1,'date':'2026-08-15','mode':'isolated-compose-synthetic-remote-signing-task-e2e',
  'inputs':{'syntheticApkOnly':True,'testKeystoreOnly':True,'temporaryCertificatesOnly':True,
            'realCustomerDataAccessed':False,'productionCredentialsAccessed':False,'sharedDevelopmentDatabaseMutated':False},
  'isolatedComposeProject':os.environ['PROJECT'],
  'temporaryIdentity':{'tenantId':int(os.environ['TENANT_ID']),'appId':int(os.environ['APP_ID']),
                       'signingConfigId':int(os.environ['SIGNING_ID']),'taskId':int(os.environ['TASK_ID'])},
  'result':{'builderAttempt':int(os.environ['BUILDER_ATTEMPT']),'apkObjectId':int(os.environ['APK_OBJECT_ID']),
            'certificateSha256':os.environ['CERTIFICATE_SHA']},
  'checks':{'publicManagementApiCreatedRemoteSigningConfig':'passed','remoteSignerIdentityValidatedAtCreation':'passed',
            'signingModePersistedWithoutKeystore':'passed','realBuilderRemoteSigningTaskSucceeded':'passed',
            'nonCapableBuilderCouldNotClaimRemoteTask':'passed',
            'finalAPKCertificateAndPackageValidated':'passed','channelAssetValidated':'passed',
            'secretLogLeakScan':'passed','isolatedComposeDestroyed':'pending'},
  'limitations':['The remote signer is a local apksigner-backed fixture, not a real HSM.'],
}
path=pathlib.Path(os.environ['EVIDENCE_PATH']);path.write_text(json.dumps(payload,ensure_ascii=False,indent=2)+'\n')
PY
chmod 0600 "$evidence"

compose down -v --remove-orphans
stack_started=false
containers=$(docker ps -aq --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
volumes=$(docker volume ls -q --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
networks=$(docker network ls -q --filter "label=com.docker.compose.project=$project" | wc -l | tr -d ' ')
[[ $containers == 0 && $volumes == 0 && $networks == 0 ]]

EVIDENCE_PATH="$evidence" python3 - <<'PY'
import json,os,pathlib
p=pathlib.Path(os.environ['EVIDENCE_PATH']);d=json.loads(p.read_text());d['checks']['isolatedComposeDestroyed']='passed'
d['cleanup']={'isolatedContainersRemaining':0,'isolatedVolumesRemaining':0,'isolatedNetworksRemaining':0}
p.write_text(json.dumps(d,ensure_ascii=False,indent=2)+'\n')
PY
chmod 0600 "$evidence"

echo '远程 APK 签名任务 E2E 通过：API持久化、真实Worker、最终APK和隔离清理均已验证。'
echo "证据: $evidence"
