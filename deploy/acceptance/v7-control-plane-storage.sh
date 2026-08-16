#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
gateway_port=${APPFORGE_CONTROL_PLANE_ACCEPTANCE_PORT:-19445}
suffix="$(date +%s)-$$"
temporary=$(mktemp -d)
api_container="appforge-control-plane-acceptance-api-$$"
agent_container="appforge-control-plane-acceptance-agent-$$"
tls_volume="appforge-control-plane-tls-$$"
agent_state_volume="appforge-control-plane-agent-state-$$"
agent_secret_volume="appforge-control-plane-agent-secret-$$"
config_key="/appforge/control-plane-storage-acceptance-$$/config"
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
api_image=${APPFORGE_CONTROL_PLANE_API_IMAGE:-appforge-dev-admin-api}
network=${APPFORGE_DEV_NETWORK:-appforge-dev}
mysql=(docker exec appforge-mysql mysql -uappforge -pappforge_dev_password -D appforge -N -B)
tenant_id=''
task_id=''
agent_id=''

cleanup() {
  docker rm -f "$agent_container" "$api_container" >/dev/null 2>&1 || true
  docker exec appforge-etcd etcdctl --endpoints=http://127.0.0.1:2379 del "$config_key" >/dev/null 2>&1 || true
  if [[ $agent_id =~ ^[1-9][0-9]*$ && $task_id =~ ^[1-9][0-9]*$ ]]; then
    for attempt in 1 2 3 4; do
      docker exec appforge-redis redis-cli DEL \
        "appforge:local-agent:artifact:task:$agent_id:$task_id:$attempt" \
        "appforge:local-agent:artifact:upload:$agent_id:$task_id:$attempt:built_apk" \
        "appforge:local-agent:artifact:upload:$agent_id:$task_id:$attempt:build_log" >/dev/null 2>&1 || true
    done
  fi
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
  docker volume rm "$tls_volume" "$agent_state_volume" "$agent_secret_volume" >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT

mkdir -p "$temporary/tls" "$temporary/fixture" "$temporary/result"

docker compose -f "$repo_root/deploy/docker-compose.dev.yml" build admin-api core-rpc >/dev/null
docker compose -f "$repo_root/deploy/docker-compose.dev.yml" up -d --no-deps --force-recreate core-rpc >/dev/null
for _ in $(seq 1 60); do
  core_health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' appforge-core-rpc 2>/dev/null || true)
  [[ $core_health == healthy ]] && break
  sleep 1
done
[[ ${core_health:-} == healthy ]] || { docker logs appforge-core-rpc >&2; echo '最新 Core RPC 未健康启动' >&2; exit 1; }
docker build -f "$repo_root/deploy/docker/local-agent.Dockerfile" -t "$agent_image" "$repo_root" >/dev/null

openssl ecparam -name prime256v1 -genkey -noout -out "$temporary/tls/ca.key"
openssl req -x509 -new -key "$temporary/tls/ca.key" -days 1 \
  -subj '/CN=AppForge CONTROL_PLANE_STORAGE Acceptance CA' \
  -addext 'basicConstraints=critical,CA:TRUE' \
  -addext 'keyUsage=critical,keyCertSign,cRLSign' \
  -out "$temporary/tls/ca.crt" >/dev/null 2>&1

openssl ecparam -name prime256v1 -genkey -noout -out "$temporary/tls/server.key"
openssl req -new -key "$temporary/tls/server.key" -subj '/CN=localhost' -out "$temporary/tls/server.csr" >/dev/null 2>&1
cat >"$temporary/tls/server.ext" <<'EOF'
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:localhost,DNS:host.docker.internal,IP:127.0.0.1
EOF
openssl x509 -req -in "$temporary/tls/server.csr" -CA "$temporary/tls/ca.crt" -CAkey "$temporary/tls/ca.key" \
  -CAcreateserial -days 1 -extfile "$temporary/tls/server.ext" -out "$temporary/tls/server.crt" >/dev/null 2>&1

issue_client_certificate() {
  local name=$1
  openssl ecparam -name prime256v1 -genkey -noout -out "$temporary/tls/$name.key"
  openssl req -new -key "$temporary/tls/$name.key" -subj "/CN=$name" -out "$temporary/tls/$name.csr" >/dev/null 2>&1
  cat >"$temporary/tls/$name.ext" <<'EOF'
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature
extendedKeyUsage=clientAuth
EOF
  openssl x509 -req -in "$temporary/tls/$name.csr" -CA "$temporary/tls/ca.crt" -CAkey "$temporary/tls/ca.key" \
    -CAcreateserial -days 1 -extfile "$temporary/tls/$name.ext" -out "$temporary/tls/$name.crt" >/dev/null 2>&1
}
issue_client_certificate agent
issue_client_certificate wrong-agent

docker run --rm --user 0 -v "$temporary/fixture:/out" --entrypoint sh "$agent_image" -lc '
set -eu
cat >/tmp/AndroidManifest.xml <<"EOF"
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.appforge.controlplane.acceptance" android:versionCode="1" android:versionName="1.0">
  <uses-sdk android:minSdkVersion="21" />
  <application android:allowBackup="false" android:label="Control Plane Acceptance" />
</manifest>
EOF
aapt package -f -M /tmp/AndroidManifest.xml -I /usr/share/android-framework-res/framework-res.apk -F /out/source.apk
keytool -genkeypair -noprompt -alias release -keyalg RSA -keysize 2048 -validity 30 \
  -dname "CN=AppForge CONTROL_PLANE_STORAGE Acceptance,O=AppForge,C=CN" \
  -keystore /out/release.jks -storepass changeit -keypass changeit >/dev/null 2>&1
keytool -exportcert -alias release -keystore /out/release.jks -storepass changeit -file /out/release.der >/dev/null 2>&1
chmod 0644 /out/source.apk /out/release.jks /out/release.der
'

source_size=$(wc -c <"$temporary/fixture/source.apk" | tr -d ' ')
keystore_size=$(wc -c <"$temporary/fixture/release.jks" | tr -d ' ')
source_sha=$(shasum -a 256 "$temporary/fixture/source.apk" | awk '{print $1}')
keystore_sha=$(shasum -a 256 "$temporary/fixture/release.jks" | awk '{print $1}')
certificate_sha=$(shasum -a 256 "$temporary/fixture/release.der" | awk '{print $1}')
client_serial=$(openssl x509 -in "$temporary/tls/agent.crt" -noout -serial | cut -d= -f2 | tr '[:upper:]' '[:lower:]' | sed 's/^0*//')
client_fingerprint=$(openssl x509 -in "$temporary/tls/agent.crt" -outform DER | openssl dgst -sha256 -r | awk '{print $1}')
client_pem_base64=$(base64 <"$temporary/tls/agent.crt" | tr -d '\n')

tenant_id=$("${mysql[@]}" -e "INSERT INTO sys_tenant (tenant_code,tenant_name,enabled,expire_time,remark,create_by,create_times,update_by,update_times) VALUES ('cp-storage-$suffix','CONTROL_PLANE_STORAGE 临时验收租户',1,0,'自动清理的临时验收租户','acceptance',UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000,'acceptance',UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000); SELECT LAST_INSERT_ID();" | tail -1)
[[ $tenant_id =~ ^[1-9][0-9]*$ ]] || { echo '创建临时租户失败' >&2; exit 1; }

source_key="tenants/$tenant_id/source-apk/acceptance-$suffix/source.apk"
keystore_key="tenants/$tenant_id/keystore/acceptance-$suffix/release.jks"
docker run --rm --network "$network" -v "$temporary/fixture:/fixture:ro" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
set -eu
mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
mc cp /fixture/source.apk local/appforge/$source_key >/dev/null
mc cp /fixture/release.jks local/appforge/$keystore_key >/dev/null
"

seed=$(
  "${mysql[@]}" -e "
INSERT INTO t_tenant_subscription (tenant_id,plan_id,plan_version,status,source,current_period_start,current_period_end) VALUES ($tenant_id,2,1,1,3,CURRENT_TIMESTAMP(3),DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY));
SET @subscription_id=LAST_INSERT_ID();
INSERT INTO t_tenant_entitlement (tenant_id,source_type,source_id,plan_id,plan_version,builds_per_cycle,max_build_concurrency,storage_bytes,max_upload_bytes,team_seats,api_rate_limit,charge_failed_build,charge_cache_hit,charge_retry_build,valid_from,valid_until,status,revision) VALUES ($tenant_id,3,@subscription_id,2,1,-1,4,-1,-1,10,1000,0,1,1,CURRENT_TIMESTAMP(3),DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY),1,1);
INSERT INTO t_app_application (tenant_id,app_code,app_name,package_name,api_host,status,create_by) VALUES ($tenant_id,'cp-storage-$suffix','CONTROL_PLANE_STORAGE Acceptance','com.appforge.controlplane.acceptance','https://api.acceptance.invalid',1,0);
SET @app_id=LAST_INSERT_ID();
INSERT INTO t_storage_object (tenant_id,app_id,object_type,object_key,original_name,content_type,size_bytes,sha256,status,create_by) VALUES
($tenant_id,@app_id,1,'$source_key','source.apk','application/vnd.android.package-archive',$source_size,'$source_sha',3,0),
($tenant_id,@app_id,2,'$keystore_key','release.jks','application/octet-stream',$keystore_size,'$keystore_sha',3,0);
SET @source_id=LAST_INSERT_ID(); SET @keystore_id=@source_id+1;
INSERT INTO t_app_version (tenant_id,app_id,version_code,version_name,source_apk_object_id,source_apk_sha256,status,create_by) VALUES ($tenant_id,@app_id,1,'1.0',@source_id,'$source_sha',2,0);
SET @version_id=LAST_INSERT_ID();
INSERT INTO t_promotion_channel (tenant_id,app_id,channel_code,channel_name,landing_url,status,create_by) VALUES ($tenant_id,@app_id,'cp-$tenant_id','Control Plane Acceptance','https://acceptance.invalid',1,0);
SET @channel_id=LAST_INSERT_ID();
INSERT INTO t_app_signing_config (tenant_id,app_id,name,keystore_object_id,keystore_object_key,key_alias,certificate_sha256,secret_ref,status,create_by) VALUES ($tenant_id,@app_id,'CONTROL_PLANE_STORAGE Acceptance',@keystore_id,'$keystore_key','release','$certificate_sha','local-file:///acceptance.json',1,0);
SET @signing_id=LAST_INSERT_ID();
INSERT INTO t_build_task (tenant_id,app_id,version_id,channel_id,signing_config_id,channel_code,version_code,version_name,source_apk_object_id,pool_code,status,builder_attempt,priority,queued_at,create_by) VALUES ($tenant_id,@app_id,@version_id,@channel_id,@signing_id,'cp-$tenant_id',1,'1.0',@source_id,'local-acceptance-$tenant_id','PENDING',0,1000,CURRENT_TIMESTAMP(3),0);
SET @task_id=LAST_INSERT_ID();
INSERT INTO t_quota_reservation (tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,period_key,status,expires_at,confirmed_at) VALUES ($tenant_id,'build.count',1,'build',@task_id,CONCAT('build:',@task_id),DATE_FORMAT(CURRENT_DATE,'%Y-%m'),2,DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY),CURRENT_TIMESTAMP(3));
INSERT INTO t_local_agent (tenant_id,agent_code,agent_name,pool_code,status,drain_status,protocol_version,agent_version,artifact_mode,allowed_app_ids,certificate_serial,create_by) VALUES ($tenant_id,'cp-agent-$suffix','CONTROL_PLANE_STORAGE Acceptance Agent','local-acceptance-$tenant_id',2,1,3,'1.1.0',1,JSON_ARRAY(@app_id),'$client_serial',0);
SET @agent_id=LAST_INSERT_ID();
INSERT INTO t_local_agent_certificate (tenant_id,agent_id,serial_number,fingerprint_sha256,certificate_pem,status,not_before,not_after) VALUES ($tenant_id,@agent_id,'$client_serial','$client_fingerprint',FROM_BASE64('$client_pem_base64'),1,DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 MINUTE),DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY));
INSERT INTO t_local_agent_capability (tenant_id,agent_id,capability_key,capability_value) VALUES ($tenant_id,@agent_id,'apk','true'),($tenant_id,@agent_id,'max_concurrency','1');
SELECT @app_id,@task_id,@agent_id;"
)
read -r app_id task_id agent_id <<<"$seed"
[[ $app_id =~ ^[1-9][0-9]*$ && $task_id =~ ^[1-9][0-9]*$ && $agent_id =~ ^[1-9][0-9]*$ ]] || { echo '创建临时业务数据失败' >&2; exit 1; }

sed \
  -e 's/^Port: 8888$/Port: 18888/' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s/^  Enabled: false/  Enabled: true/' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ServerCertificate:.*#  ServerCertificate: /etc/appforge-acceptance/server.crt#' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ServerPrivateKey:.*#  ServerPrivateKey: /etc/appforge-acceptance/server.key#' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ClientCACertificate:.*#  ClientCACertificate: /etc/appforge-acceptance/ca.crt#' \
  "$repo_root/deploy/etcd/admin-api.yaml" >"$temporary/admin-api.yaml"

docker volume create "$tls_volume" >/dev/null
docker run --rm --user 0 -v "$tls_volume:/target" -v "$temporary/tls:/source:ro" alpine:3.23 sh -lc \
  'cp /source/ca.crt /source/server.crt /source/server.key /target/; chown -R 65532:65532 /target; chmod 0700 /target; chmod 0600 /target/*'
docker exec -i appforge-etcd etcdctl --endpoints=http://127.0.0.1:2379 put "$config_key" <"$temporary/admin-api.yaml" >/dev/null
docker run -d --name "$api_container" --network "$network" -p "$gateway_port:9443" \
  -v "$tls_volume:/etc/appforge-acceptance:ro" "$api_image" -etcd etcd:2379 -config "$config_key" >/dev/null

for _ in $(seq 1 60); do
  if code=$(curl -sS -o /dev/null -w '%{http_code}' --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/agent.crt" --key "$temporary/tls/agent.key" "https://localhost:$gateway_port/v1/heartbeat"); then
    [[ $code != 000 ]] && break
  fi
  sleep 1
done
[[ ${code:-000} != 000 ]] || { docker logs "$api_container" >&2; echo 'mTLS Gateway 未启动' >&2; exit 1; }

auth_timestamp=$(( $(date +%s) * 1000 ))
next_auth() {
  auth_timestamp=$((auth_timestamp + 1))
  auth_nonce=$(openssl rand -hex 16)
}
agent_post() {
  local path=$1 body=$2 output=$3
  curl -sS --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/agent.crt" --key "$temporary/tls/agent.key" \
    -H 'Content-Type: application/json' -d "$body" "https://localhost:$gateway_port$path" >"$output"
}

next_auth
agent_post /v1/heartbeat "{\"auth\":{\"agent_id\":$agent_id,\"nonce\":\"$auth_nonce\",\"timestamp\":$auth_timestamp},\"agent_version\":\"1.1.0\",\"protocol_version\":3,\"capabilities\":[{\"capability_key\":\"apk\",\"capability_value\":\"true\"},{\"capability_key\":\"max_concurrency\",\"capability_value\":\"1\"}]}" "$temporary/heartbeat.json"
if ! HEARTBEAT_FILE="$temporary/heartbeat.json" python3 -c "import json,os; p=json.load(open(os.environ['HEARTBEAT_FILE'])); assert p.get('base',{}).get('code') in (0,200)"; then
  cat "$temporary/heartbeat.json" >&2
  docker logs "$api_container" >&2 || true
  echo 'Local Agent 心跳认证失败' >&2
  exit 1
fi
next_auth
agent_post /v1/claim "{\"auth\":{\"agent_id\":$agent_id,\"nonce\":\"$auth_nonce\",\"timestamp\":$auth_timestamp},\"lease_seconds\":120}" "$temporary/claim.json"

if ! CLAIM_FILE="$temporary/claim.json" python3 -c "import json,os; p=json.load(open(os.environ['CLAIM_FILE'])); assert p.get('task')"; then
  cat "$temporary/claim.json" >&2
  docker logs "$api_container" >&2 || true
  "${mysql[@]}" -e "SELECT id,status,builder_id,builder_attempt,lease_until FROM t_build_task WHERE id=$task_id; SELECT id,status,drain_status,protocol_version,artifact_mode,allowed_app_ids,last_request_at FROM t_local_agent WHERE id=$agent_id; SELECT tenant_id,status,max_build_concurrency,valid_until FROM t_tenant_entitlement WHERE tenant_id=$tenant_id" >&2 || true
  echo 'Local Agent 未领取到临时任务' >&2
  exit 1
fi

CLAIM_FILE="$temporary/claim.json" python3 - <<'PY'
import json, os
p = json.load(open(os.environ['CLAIM_FILE']))
assert p['task']['builder_attempt'] == 1
assert p['artifact_mode'] == 1
assert p['bundle']['schema_version'] == 3
assert all(x.get('download_path', '').startswith('/v1/artifacts/download/') for x in p['bundle']['inputs'])
assert all(x.get('upload_path', '').startswith('/v1/artifacts/upload/') for x in p['bundle']['outputs'])
assert 'object_key' not in json.dumps(p)
PY
source_path=$(CLAIM_FILE="$temporary/claim.json" python3 -c "import json,os; p=json.load(open(os.environ['CLAIM_FILE'])); print(next(x['download_path'] for x in p['bundle']['inputs'] if x['role']=='source_apk'))")
keystore_path=$(CLAIM_FILE="$temporary/claim.json" python3 -c "import json,os; p=json.load(open(os.environ['CLAIM_FILE'])); print(next(x['download_path'] for x in p['bundle']['inputs'] if x['role']=='keystore'))")
apk_upload_path=$(CLAIM_FILE="$temporary/claim.json" python3 -c "import json,os; p=json.load(open(os.environ['CLAIM_FILE'])); print(next(x['upload_path'] for x in p['bundle']['outputs'] if x['role']=='built_apk'))")

wrong_code=$(curl -sS -o /dev/null -w '%{http_code}' --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/wrong-agent.crt" --key "$temporary/tls/wrong-agent.key" "https://localhost:$gateway_port$source_path")
[[ $wrong_code == 403 ]] || { echo "错误证书未被拒绝: HTTP $wrong_code" >&2; exit 1; }
replay_code=$(curl -sS -o /dev/null -w '%{http_code}' --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/agent.crt" --key "$temporary/tls/agent.key" "https://localhost:$gateway_port$source_path")
[[ $replay_code == 404 ]] || { echo "错误证书消费后的票据仍可使用: HTTP $replay_code" >&2; exit 1; }
curl -sS --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/agent.crt" --key "$temporary/tls/agent.key" "https://localhost:$gateway_port$keystore_path" -o "$temporary/result/downloaded.jks"
cmp "$temporary/fixture/release.jks" "$temporary/result/downloaded.jks"
replay_code=$(curl -sS -o /dev/null -w '%{http_code}' --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/agent.crt" --key "$temporary/tls/agent.key" "https://localhost:$gateway_port$keystore_path")
[[ $replay_code == 404 ]] || { echo "下载票据可重放: HTTP $replay_code" >&2; exit 1; }

printf 'control-plane-A' >"$temporary/result/candidate.apk"
candidate_size=$(wc -c <"$temporary/result/candidate.apk" | tr -d ' ')
candidate_sha=$(shasum -a 256 "$temporary/result/candidate.apk" | awk '{print $1}')
curl -sS --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/agent.crt" --key "$temporary/tls/agent.key" \
  -X PUT -H "X-AppForge-Sha256: $candidate_sha" -H 'Content-Type: application/vnd.android.package-archive' \
  --data-binary @"$temporary/result/candidate.apk" "https://localhost:$gateway_port$apk_upload_path" >"$temporary/upload.json"
uploaded_id=$(UPLOAD_FILE="$temporary/upload.json" python3 -c "import json,os; print(json.load(open(os.environ['UPLOAD_FILE']))['reference'].split('://',1)[1])")
uploaded_key=$("${mysql[@]}" -e "SELECT object_key FROM t_storage_object WHERE id=$uploaded_id AND tenant_id=$tenant_id")
printf 'control-plane-B' >"$temporary/result/tampered.apk"
docker run --rm --network "$network" -v "$temporary/result:/result:ro" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
set -eu
mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
mc cp /result/tampered.apk local/appforge/$uploaded_key >/dev/null
"
next_auth
tamper_code=$(curl -sS -o "$temporary/tamper.json" -w '%{http_code}' --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/agent.crt" --key "$temporary/tls/agent.key" \
  -H 'Content-Type: application/json' -d "{\"auth\":{\"agent_id\":$agent_id,\"nonce\":\"$auth_nonce\",\"timestamp\":$auth_timestamp},\"task_id\":$task_id,\"builder_attempt\":1,\"apk_reference\":\"storage-object://$uploaded_id\",\"apk_sha256\":\"$candidate_sha\",\"apk_size\":$candidate_size}" \
  "https://localhost:$gateway_port/v1/tasks/complete")
[[ $tamper_code == 412 ]] || { cat "$temporary/tamper.json" >&2; echo "控制面未拒绝被篡改的上传对象: HTTP $tamper_code" >&2; exit 1; }

"${mysql[@]}" -e "UPDATE t_build_task SET lease_until=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE id=$task_id AND builder_attempt=1; UPDATE t_build_slot_lease SET lease_until=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE task_id=$task_id AND builder_attempt=1;" >/dev/null

docker volume create "$agent_state_volume" >/dev/null
docker volume create "$agent_secret_volume" >/dev/null
cat >"$temporary/state.json" <<EOF
{
  "agentId": $agent_id,
  "gatewayUrl": "https://host.docker.internal:$gateway_port",
  "certificate": "/state/agent.crt",
  "privateKey": "/state/agent.key",
  "clientCa": "/state/ca.crt",
  "gatewayCa": "/state/ca.crt",
  "protocol": 3,
  "agentVersion": "1.1.0",
  "lastTimestamp": $auth_timestamp
}
EOF
cat >"$temporary/acceptance.json" <<'EOF'
{"keystorePassword":"changeit","keyPassword":"changeit"}
EOF
docker run --rm --user 0 -v "$agent_state_volume:/state" -v "$agent_secret_volume:/secrets" -v "$temporary:/source:ro" alpine:3.23 sh -lc '
cp /source/state.json /source/tls/agent.crt /source/tls/agent.key /source/tls/ca.crt /state/
cp /source/acceptance.json /secrets/
chown -R 65532:65532 /state /secrets
chmod 0700 /state /secrets
chmod 0600 /state/* /secrets/*
'
docker run -d --name "$agent_container" --network "$network" -v "$agent_state_volume:/state" -v "$agent_secret_volume:/secrets" \
  "$agent_image" run --state-dir /state --secret-root /secrets --poll 1s --lease-seconds 120 --max-concurrency 1 >/dev/null

status=''
for _ in $(seq 1 120); do
  status=$("${mysql[@]}" -e "SELECT status FROM t_build_task WHERE id=$task_id")
  [[ $status == SUCCESS || $status == FAILED ]] && break
  sleep 1
done
if [[ $status != SUCCESS ]]; then
  docker logs "$agent_container" >&2 || true
  docker logs "$api_container" >&2 || true
  "${mysql[@]}" -e "SELECT id,status,builder_id,builder_attempt,error_message FROM t_build_task WHERE id=$task_id" >&2 || true
  echo "Local Agent 未完成 CONTROL_PLANE_STORAGE 构建: $status" >&2
  exit 1
fi

result=$("${mysql[@]}" -e "SELECT builder_attempt,apk_object_id,log_object_id,apk_url,log_url,apk_sha256,apk_size FROM t_build_task WHERE id=$task_id")
read -r final_attempt apk_object_id log_object_id apk_reference log_reference apk_sha apk_size <<<"$result"
[[ $final_attempt == 2 && $apk_object_id =~ ^[1-9][0-9]*$ && $log_object_id =~ ^[1-9][0-9]*$ ]] || { echo "任务恢复或产物绑定异常: $result" >&2; exit 1; }
[[ $apk_reference == "storage-object://$apk_object_id" && $log_reference == "storage-object://$log_object_id" ]] || { echo "任务未保存规范控制面对象引用: $result" >&2; exit 1; }
object_result=$("${mysql[@]}" -e "SELECT object_key,size_bytes,sha256,status FROM t_storage_object WHERE id=$apk_object_id AND tenant_id=$tenant_id")
read -r apk_key stored_size stored_sha stored_status <<<"$object_result"
[[ $stored_status == 3 && $stored_size == "$apk_size" && $stored_sha == "$apk_sha" ]] || { echo "Core 产物绑定元数据不一致: $object_result" >&2; exit 1; }
docker run --rm --network "$network" -v "$temporary/result:/result" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
set -eu
mc alias set local http://minio:9000 appforge appforge_dev_minio >/dev/null
mc cp local/appforge/$apk_key /result/final.apk >/dev/null
"
final_sha=$(shasum -a 256 "$temporary/result/final.apk" | awk '{print $1}')
[[ $final_sha == "$apk_sha" ]] || { echo '最终 APK 字节摘要与 Core 不一致' >&2; exit 1; }
docker run --rm --user 0 -v "$temporary/result:/result:ro" --entrypoint sh "$agent_image" -lc \
  "apksigner verify --verbose --print-certs /result/final.apk >/dev/null && aapt dump badging /result/final.apk | grep -q \"package: name='com.appforge.controlplane.acceptance'\""

echo "通过: CONTROL_PLANE_STORAGE mTLS 单次票据、证书绑定、防重放、篡改拒绝、attempt fencing、真实 APK/Keystore 下载、APK/日志上传、控制面 SHA 复核与 Core 绑定"
