#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
gateway_port=${APPFORGE_CUSTOMER_STORAGE_ACCEPTANCE_PORT:-19446}
suffix="$(date +%s)-$$"
temporary=$(mktemp -d)
api_container="appforge-customer-storage-api-$$"
agent_container="appforge-customer-storage-agent-$$"
customer_minio="appforge-customer-minio-$$"
tls_volume="appforge-customer-tls-$$"
agent_state_volume="appforge-customer-agent-state-$$"
agent_secret_volume="appforge-customer-agent-secret-$$"
fixture_volume="appforge-customer-fixture-$$"
customer_data_volume="appforge-customer-data-$$"
config_key="/appforge/customer-storage-acceptance-$$/config"
agent_image=${APPFORGE_LOCAL_AGENT_IMAGE:-appforge-local-agent:dev}
api_image=${APPFORGE_CUSTOMER_STORAGE_API_IMAGE:-appforge-dev-admin-api}
network=${APPFORGE_DEV_NETWORK:-appforge-dev}
customer_bucket="appforge-customer-$suffix"
customer_access="customer-$suffix"
customer_secret="synthetic-customer-secret-$suffix"
customer_root_access="root-$suffix"
customer_root_secret="synthetic-root-secret-$suffix"
mysql=(docker exec appforge-mysql mysql -uappforge -pappforge_dev_password -D appforge -N -B)
tenant_id=''
agent_id=''

cleanup() {
  docker rm -f "$agent_container" "$api_container" "$customer_minio" >/dev/null 2>&1 || true
  docker exec appforge-etcd etcdctl --endpoints=http://127.0.0.1:2379 del "$config_key" >/dev/null 2>&1 || true
  if [[ -n $tenant_id && $tenant_id =~ ^[1-9][0-9]*$ ]]; then
    tables=$("${mysql[@]}" -e "SELECT DISTINCT TABLE_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA='appforge' AND COLUMN_NAME='tenant_id' AND TABLE_NAME<>'sys_tenant'" 2>/dev/null || true)
    for table in $tables; do
      [[ $table =~ ^[a-zA-Z0-9_]+$ ]] || continue
      "${mysql[@]}" -e "DELETE FROM \`$table\` WHERE tenant_id=$tenant_id" >/dev/null 2>&1 || true
    done
    "${mysql[@]}" -e "DELETE FROM sys_tenant WHERE id=$tenant_id" >/dev/null 2>&1 || true
  fi
  docker volume rm "$tls_volume" "$agent_state_volume" "$agent_secret_volume" "$fixture_volume" "$customer_data_volume" >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT

mkdir -p "$temporary/tls" "$temporary/fixture" "$temporary/result"

docker compose -f "$repo_root/deploy/docker-compose.dev.yml" up -d --force-recreate mysql-migrate >/dev/null
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
  -subj '/CN=AppForge CUSTOMER_STORAGE Acceptance CA' \
  -addext 'basicConstraints=critical,CA:TRUE' -addext 'keyUsage=critical,keyCertSign,cRLSign' \
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
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.appforge.customer.acceptance" android:versionCode="1" android:versionName="1.0">
  <uses-sdk android:minSdkVersion="21" />
  <application android:allowBackup="false" android:label="Customer Storage Acceptance" />
</manifest>
EOF
aapt package -f -M /tmp/AndroidManifest.xml -I /usr/share/android-framework-res/framework-res.apk -F /out/source.apk
keytool -genkeypair -noprompt -alias release -keyalg RSA -keysize 2048 -validity 30 \
  -dname "CN=AppForge CUSTOMER_STORAGE Acceptance,O=AppForge,C=CN" \
  -keystore /out/release.jks -storepass changeit -keypass changeit >/dev/null 2>&1
keytool -exportcert -alias release -keystore /out/release.jks -storepass changeit -file /out/release.der >/dev/null 2>&1
chmod 0600 /out/source.apk /out/release.jks /out/release.der
'
source_sha=$(shasum -a 256 "$temporary/fixture/source.apk" | awk '{print $1}')
source_size=$(wc -c <"$temporary/fixture/source.apk" | tr -d ' ')
keystore_sha=$(shasum -a 256 "$temporary/fixture/release.jks" | awk '{print $1}')
keystore_size=$(wc -c <"$temporary/fixture/release.jks" | tr -d ' ')
certificate_sha=$(shasum -a 256 "$temporary/fixture/release.der" | awk '{print $1}')
client_serial=$(openssl x509 -in "$temporary/tls/agent.crt" -noout -serial | cut -d= -f2 | tr '[:upper:]' '[:lower:]' | sed 's/^0*//')
client_fingerprint=$(openssl x509 -in "$temporary/tls/agent.crt" -outform DER | openssl dgst -sha256 -r | awk '{print $1}')
client_pem_base64=$(base64 <"$temporary/tls/agent.crt" | tr -d '\n')

tenant_id=$("${mysql[@]}" -e "INSERT INTO sys_tenant (tenant_code,tenant_name,enabled,expire_time,remark,create_by,create_times,update_by,update_times) VALUES ('customer-storage-$suffix','CUSTOMER_STORAGE 临时验收租户',1,0,'仅含合成数据并自动清理','acceptance',UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000,'acceptance',UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000); SELECT LAST_INSERT_ID();" | tail -1)
[[ $tenant_id =~ ^[1-9][0-9]*$ ]] || { echo '创建临时租户失败' >&2; exit 1; }
agent_code="customer-agent-$suffix"
customer_prefix="tenants/$tenant_id/agents/$agent_code"
customer_storage_ref="local-file:///customer-storage.json#$customer_prefix"

seed=$("${mysql[@]}" -e "
INSERT INTO t_tenant_subscription (tenant_id,plan_id,plan_version,status,source,current_period_start,current_period_end) VALUES ($tenant_id,2,1,1,3,CURRENT_TIMESTAMP(3),DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY));
SET @subscription_id=LAST_INSERT_ID();
INSERT INTO t_tenant_entitlement (tenant_id,source_type,source_id,plan_id,plan_version,builds_per_cycle,max_build_concurrency,storage_bytes,max_upload_bytes,team_seats,api_rate_limit,charge_failed_build,charge_cache_hit,charge_retry_build,valid_from,valid_until,status,revision) VALUES ($tenant_id,3,@subscription_id,2,1,-1,4,-1,-1,10,1000,0,1,1,CURRENT_TIMESTAMP(3),DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY),1,1);
INSERT INTO t_app_application (tenant_id,app_code,app_name,package_name,api_host,status,create_by) VALUES ($tenant_id,'customer-storage-$suffix','CUSTOMER_STORAGE Acceptance','com.appforge.customer.acceptance','https://api.acceptance.invalid',1,0);
SET @app_id=LAST_INSERT_ID();
INSERT INTO t_local_agent (tenant_id,agent_code,agent_name,pool_code,status,drain_status,protocol_version,agent_version,artifact_mode,customer_storage_ref,allowed_app_ids,certificate_serial,create_by) VALUES ($tenant_id,'$agent_code','CUSTOMER_STORAGE Acceptance Agent','customer-pool-$tenant_id',2,1,3,'1.1.0',2,'$customer_storage_ref',JSON_ARRAY(@app_id),'$client_serial',0);
SET @agent_id=LAST_INSERT_ID();
INSERT INTO t_local_agent_certificate (tenant_id,agent_id,serial_number,fingerprint_sha256,certificate_pem,status,not_before,not_after) VALUES ($tenant_id,@agent_id,'$client_serial','$client_fingerprint',FROM_BASE64('$client_pem_base64'),1,DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 MINUTE),DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY));
INSERT INTO t_local_agent_capability (tenant_id,agent_id,capability_key,capability_value) VALUES ($tenant_id,@agent_id,'apk','true'),($tenant_id,@agent_id,'max_concurrency','1');
SELECT @app_id,@agent_id;"
)
read -r app_id agent_id <<<"$seed"
[[ $app_id =~ ^[1-9][0-9]*$ && $agent_id =~ ^[1-9][0-9]*$ ]] || { echo '创建临时 Agent 数据失败' >&2; exit 1; }

docker volume create "$customer_data_volume" >/dev/null
docker run -d --name "$customer_minio" --network "$network" -v "$customer_data_volume:/data" \
  -e MINIO_ROOT_USER="$customer_root_access" -e MINIO_ROOT_PASSWORD="$customer_root_secret" \
  minio/minio:RELEASE.2025-04-22T22-12-26Z server /data >/dev/null
cat >"$temporary/customer-policy.json" <<EOF
{"Version":"2012-10-17","Statement":[
 {"Effect":"Allow","Action":["s3:GetBucketLocation"],"Resource":["arn:aws:s3:::$customer_bucket"]},
 {"Effect":"Allow","Action":["s3:GetObject","s3:PutObject"],"Resource":["arn:aws:s3:::$customer_bucket/$customer_prefix/*"]}
]}
EOF
for _ in $(seq 1 60); do
  if docker run --rm --network "$network" minio/mc:RELEASE.2025-04-16T18-13-26Z \
    alias set customer "http://$customer_minio:9000" "$customer_root_access" "$customer_root_secret" >/dev/null 2>&1; then break; fi
  sleep 1
done
docker run --rm --network "$network" -v "$temporary/customer-policy.json:/policy.json:ro" --entrypoint /bin/sh \
  minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
set -eu
mc alias set customer http://$customer_minio:9000 '$customer_root_access' '$customer_root_secret' >/dev/null
mc mb customer/$customer_bucket >/dev/null
mc anonymous set none customer/$customer_bucket >/dev/null
mc admin user add customer '$customer_access' '$customer_secret' >/dev/null
mc admin policy create customer appforge-customer-prefix /policy.json >/dev/null
mc admin policy attach customer appforge-customer-prefix --user '$customer_access' >/dev/null
"
if docker run --rm --network "$network" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
mc alias set restricted http://$customer_minio:9000 '$customer_access' '$customer_secret' >/dev/null
printf forbidden | mc pipe restricted/$customer_bucket/outside-prefix/forbidden >/dev/null 2>&1
"; then
  echo '客户测试凭据可写登记前缀之外对象' >&2
  exit 1
fi

sed \
  -e 's/^Port: 8888$/Port: 18889/' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s/^  Enabled: false/  Enabled: true/' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ServerCertificate:.*#  ServerCertificate: /etc/appforge-acceptance/server.crt#' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ServerPrivateKey:.*#  ServerPrivateKey: /etc/appforge-acceptance/server.key#' \
  -e '/^LocalAgentGateway:/,/^OfflineLicense:/ s#^  ClientCACertificate:.*#  ClientCACertificate: /etc/appforge-acceptance/ca.crt#' \
  "$repo_root/deploy/etcd/admin-api.yaml" >"$temporary/admin-api.yaml"
docker volume create "$tls_volume" >/dev/null
docker run --rm --user 0 -v "$tls_volume:/target" -v "$temporary/tls:/source:ro" alpine:3.21 sh -lc \
  'cp /source/ca.crt /source/server.crt /source/server.key /target/; chown -R 65532:65532 /target; chmod 0700 /target; chmod 0600 /target/*'
docker exec -i appforge-etcd etcdctl --endpoints=http://127.0.0.1:2379 put "$config_key" <"$temporary/admin-api.yaml" >/dev/null
docker run -d --name "$api_container" --network "$network" -p "$gateway_port:9443" \
  -v "$tls_volume:/etc/appforge-acceptance:ro" "$api_image" -etcd etcd:2379 -config "$config_key" >/dev/null
for _ in $(seq 1 60); do
  if gateway_code=$(curl -sS -o /dev/null -w '%{http_code}' --cacert "$temporary/tls/ca.crt" --cert "$temporary/tls/agent.crt" --key "$temporary/tls/agent.key" "https://localhost:$gateway_port/v1/heartbeat"); then
    [[ $gateway_code != 000 ]] && break
  fi
  sleep 1
done
[[ ${gateway_code:-000} != 000 ]] || { docker logs "$api_container" >&2; echo 'mTLS Gateway 未启动' >&2; exit 1; }

docker volume create "$agent_state_volume" >/dev/null
docker volume create "$agent_secret_volume" >/dev/null
docker volume create "$fixture_volume" >/dev/null
initial_timestamp=$(( $(date +%s) * 1000 ))
cat >"$temporary/state.json" <<EOF
{"agentId":$agent_id,"gatewayUrl":"https://host.docker.internal:$gateway_port","certificate":"/state/agent.crt","privateKey":"/state/agent.key","clientCa":"/state/ca.crt","gatewayCa":"/state/ca.crt","customerStorageRef":"$customer_storage_ref","protocol":3,"agentVersion":"1.1.0","lastTimestamp":$initial_timestamp}
EOF
cat >"$temporary/customer-storage.json" <<EOF
{"provider":"minio","endpoint":"http://$customer_minio:9000","region":"us-east-1","access_key_id":"$customer_access","access_key_secret":"$customer_secret","bucket":"$customer_bucket","prefix":"$customer_prefix"}
EOF
cat >"$temporary/signing.json" <<'EOF'
{"keystorePassword":"changeit","keyPassword":"changeit"}
EOF
docker run --rm --user 0 -v "$agent_state_volume:/state" -v "$agent_secret_volume:/secrets" -v "$fixture_volume:/fixture" -v "$temporary:/source:ro" alpine:3.21 sh -lc '
cp /source/state.json /source/tls/agent.crt /source/tls/agent.key /source/tls/ca.crt /state/
cp /source/customer-storage.json /source/signing.json /secrets/
cp /source/fixture/source.apk /source/fixture/release.jks /fixture/
chown -R 65532:65532 /state /secrets /fixture
chmod 0700 /state /secrets /fixture
chmod 0600 /state/* /secrets/* /fixture/*
'
agent_import() {
  local object_type=$1 input=$2
  docker run --rm --network "$network" -v "$agent_state_volume:/state" -v "$agent_secret_volume:/secrets:ro" -v "$fixture_volume:/fixture:ro" \
    "$agent_image" customer-storage-import --state-dir /state --secret-root /secrets --app-id "$app_id" --object-type "$object_type" --input "/fixture/$input"
}
source_import_1=$(agent_import source-apk source.apk)
source_import_2=$(agent_import source-apk source.apk)
keystore_import=$(agent_import keystore release.jks)
source_object_id=$(sed -n 's/.*object_id=\([0-9][0-9]*\).*/\1/p' <<<"$source_import_1")
source_object_retry=$(sed -n 's/.*object_id=\([0-9][0-9]*\).*/\1/p' <<<"$source_import_2")
keystore_object_id=$(sed -n 's/.*object_id=\([0-9][0-9]*\).*/\1/p' <<<"$keystore_import")
[[ $source_object_id =~ ^[1-9][0-9]*$ && $source_object_id == "$source_object_retry" && $keystore_object_id =~ ^[1-9][0-9]*$ ]] || {
  echo '客户输入登记或幂等重试失败' >&2; exit 1;
}

source_key="$customer_prefix/inputs/apps/$app_id/1/$source_sha.apk"
keystore_key="$customer_prefix/inputs/apps/$app_id/2/$keystore_sha.jks"
ownership=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_storage_object WHERE id IN ($source_object_id,$keystore_object_id) AND tenant_id=$tenant_id AND app_id=$app_id AND storage_mode=2 AND owner_agent_id=$agent_id AND status IN (2,3)")
[[ $ownership == 2 ]] || { echo '客户输入对象归属没有正确登记' >&2; exit 1; }

auth_timestamp=$("${mysql[@]}" -e "SELECT CAST(UNIX_TIMESTAMP(last_request_at)*1000 AS UNSIGNED) FROM t_local_agent WHERE id=$agent_id")
next_auth() { auth_timestamp=$((auth_timestamp + 1)); auth_nonce=$(openssl rand -hex 16); }
agent_post_code() {
  local certificate=$1 key=$2 path=$3 body=$4 output=$5
  curl -sS -o "$output" -w '%{http_code}' --cacert "$temporary/tls/ca.crt" --cert "$certificate" --key "$key" \
    -H 'Content-Type: application/json' -d "$body" "https://localhost:$gateway_port$path"
}
next_auth
wrong_code=$(agent_post_code "$temporary/tls/wrong-agent.crt" "$temporary/tls/wrong-agent.key" /v1/heartbeat \
  "{\"auth\":{\"agent_id\":$agent_id,\"nonce\":\"$auth_nonce\",\"timestamp\":$auth_timestamp},\"agent_version\":\"1.1.0\",\"protocol_version\":3}" "$temporary/wrong.json")
[[ $wrong_code == 401 ]] || { echo "错误证书未被拒绝: HTTP $wrong_code" >&2; exit 1; }
next_auth
outside_code=$(agent_post_code "$temporary/tls/agent.crt" "$temporary/tls/agent.key" /v1/customer-storage/inputs \
  "{\"auth\":{\"agent_id\":$agent_id,\"nonce\":\"$auth_nonce\",\"timestamp\":$auth_timestamp},\"app_id\":$app_id,\"object_type\":1,\"object_reference\":\"customer-object://$agent_id/tenants/$tenant_id/agents/other/input.apk\",\"original_name\":\"source.apk\",\"content_type\":\"application/vnd.android.package-archive\",\"size_bytes\":$source_size,\"sha256\":\"$source_sha\"}" "$temporary/outside.json")
[[ $outside_code == 403 ]] || { echo "越界前缀未被拒绝: HTTP $outside_code" >&2; exit 1; }
next_auth
conflict_code=$(agent_post_code "$temporary/tls/agent.crt" "$temporary/tls/agent.key" /v1/customer-storage/inputs \
  "{\"auth\":{\"agent_id\":$agent_id,\"nonce\":\"$auth_nonce\",\"timestamp\":$auth_timestamp},\"app_id\":$app_id,\"object_type\":1,\"object_reference\":\"customer-object://$agent_id/$source_key\",\"original_name\":\"source.apk\",\"content_type\":\"application/vnd.android.package-archive\",\"size_bytes\":$((source_size+1)),\"sha256\":\"$source_sha\"}" "$temporary/conflict.json")
[[ $conflict_code == 409 ]] || { echo "同一引用不同元数据未冲突: HTTP $conflict_code" >&2; exit 1; }
sleep 1

base_seed=$("${mysql[@]}" -e "
INSERT INTO t_app_version (tenant_id,app_id,version_code,version_name,source_apk_object_id,source_apk_sha256,status,create_by) VALUES ($tenant_id,$app_id,1,'1.0',$source_object_id,'$source_sha',2,0);
SET @version_id=LAST_INSERT_ID();
INSERT INTO t_promotion_channel (tenant_id,app_id,channel_code,channel_name,landing_url,status,create_by) VALUES ($tenant_id,$app_id,'customer-$tenant_id','Customer Storage Acceptance','https://acceptance.invalid',1,0);
SET @channel_id=LAST_INSERT_ID();
INSERT INTO t_app_signing_config (tenant_id,app_id,name,keystore_object_id,keystore_object_key,key_alias,certificate_sha256,secret_ref,status,create_by) VALUES ($tenant_id,$app_id,'CUSTOMER_STORAGE Acceptance',$keystore_object_id,'$keystore_key','release','$certificate_sha','local-file:///signing.json',1,0);
SET @signing_id=LAST_INSERT_ID();
SELECT @version_id,@channel_id,@signing_id;"
)
read -r version_id channel_id signing_id <<<"$base_seed"
"${mysql[@]}" -e "UPDATE t_storage_object SET status=3 WHERE id IN ($source_object_id,$keystore_object_id) AND tenant_id=$tenant_id AND storage_mode=2 AND owner_agent_id=$agent_id" >/dev/null
create_task() {
  "${mysql[@]}" -e "
INSERT INTO t_build_task (tenant_id,app_id,version_id,channel_id,signing_config_id,channel_code,version_code,version_name,source_apk_object_id,pool_code,status,builder_attempt,priority,queued_at,create_by) VALUES ($tenant_id,$app_id,$version_id,$channel_id,$signing_id,'customer-$tenant_id',1,'1.0',$source_object_id,'customer-pool-$tenant_id','PENDING',0,1000,CURRENT_TIMESTAMP(3),0);
SET @task_id=LAST_INSERT_ID();
INSERT INTO t_quota_reservation (tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,period_key,status,expires_at,confirmed_at) VALUES ($tenant_id,'build.count',1,'build',@task_id,CONCAT('build:',@task_id),DATE_FORMAT(CURRENT_DATE,'%Y-%m'),2,DATE_ADD(CURRENT_TIMESTAMP(3),INTERVAL 1 DAY),CURRENT_TIMESTAMP(3));
SELECT @task_id;" | tail -1
}

tamper_task_id=$(create_task)
printf 'tampered-synthetic-input' >"$temporary/result/tampered.apk"
docker run --rm --network "$network" -v "$temporary/result:/result:ro" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
mc alias set customer http://$customer_minio:9000 '$customer_root_access' '$customer_root_secret' >/dev/null
mc cp /result/tampered.apk customer/$customer_bucket/$source_key >/dev/null
"
docker run -d --name "$agent_container" --network "$network" -v "$agent_state_volume:/state" -v "$agent_secret_volume:/secrets:ro" \
  "$agent_image" run --state-dir /state --secret-root /secrets --poll 1s --lease-seconds 120 --max-concurrency 1 >/dev/null
for _ in $(seq 1 90); do
  tamper_status=$("${mysql[@]}" -e "SELECT status FROM t_build_task WHERE id=$tamper_task_id")
  [[ $tamper_status == FAILED ]] && break
  sleep 1
done
[[ ${tamper_status:-} == FAILED ]] || { docker logs "$agent_container" >&2; echo '篡改输入未使任务失败' >&2; exit 1; }
tamper_error=$("${mysql[@]}" -e "SELECT error_message FROM t_build_task WHERE id=$tamper_task_id")
[[ $tamper_error == *LOCAL_CUSTOMER_STORAGE_INPUT_INTEGRITY_FAILED* ]] || { echo "篡改错误不符合预期: $tamper_error" >&2; exit 1; }
docker rm -f "$agent_container" >/dev/null
docker run --rm --network "$network" -v "$fixture_volume:/fixture:ro" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
mc alias set customer http://$customer_minio:9000 '$customer_root_access' '$customer_root_secret' >/dev/null
mc cp /fixture/source.apk customer/$customer_bucket/$source_key >/dev/null
"

task_id=$(create_task)
auth_timestamp=$("${mysql[@]}" -e "SELECT GREATEST(CAST(UNIX_TIMESTAMP(last_request_at)*1000 AS UNSIGNED),CAST(UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3))*1000 AS UNSIGNED)) FROM t_local_agent WHERE id=$agent_id")
next_auth
claim_code=$(agent_post_code "$temporary/tls/agent.crt" "$temporary/tls/agent.key" /v1/claim \
  "{\"auth\":{\"agent_id\":$agent_id,\"nonce\":\"$auth_nonce\",\"timestamp\":$auth_timestamp},\"lease_seconds\":120}" "$temporary/claim.json")
[[ $claim_code == 200 ]] || { cat "$temporary/claim.json" >&2; echo 'CUSTOMER_STORAGE 手工领取失败' >&2; exit 1; }
CLAIM_FILE="$temporary/claim.json" python3 - <<'PY'
import json, os
p=json.load(open(os.environ['CLAIM_FILE']))
assert p['artifact_mode']==2 and p['task']['builder_attempt']==1
assert p['customer_storage_ref'].startswith('local-file:///')
assert all(x.get('customer_reference','').startswith('customer-object://') for x in p['bundle']['inputs'])
assert all('download_path' not in x for x in p['bundle']['inputs'])
assert all('upload_path' not in x for x in p['bundle'].get('outputs',[]))
assert 'access_key' not in json.dumps(p).lower()
PY
"${mysql[@]}" -e "UPDATE t_build_task SET lease_until=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE id=$task_id AND builder_attempt=1; UPDATE t_build_slot_lease SET lease_until=DATE_SUB(CURRENT_TIMESTAMP(3),INTERVAL 1 SECOND) WHERE task_id=$task_id AND builder_attempt=1;" >/dev/null
next_auth
stale_ref="customer-object://$agent_id/$customer_prefix/tasks/$task_id/attempts/1/built.apk"
stale_code=$(agent_post_code "$temporary/tls/agent.crt" "$temporary/tls/agent.key" /v1/tasks/complete \
  "{\"auth\":{\"agent_id\":$agent_id,\"nonce\":\"$auth_nonce\",\"timestamp\":$auth_timestamp},\"task_id\":$task_id,\"builder_attempt\":1,\"apk_reference\":\"$stale_ref\",\"apk_sha256\":\"$(printf 'a%.0s' {1..64})\",\"apk_size\":1}" "$temporary/stale.json")
[[ $stale_code == 404 ]] || { echo "过期 attempt 未被拒绝: HTTP $stale_code" >&2; exit 1; }
sleep 1

docker run -d --name "$agent_container" --network "$network" -v "$agent_state_volume:/state" -v "$agent_secret_volume:/secrets:ro" \
  "$agent_image" run --state-dir /state --secret-root /secrets --poll 1s --lease-seconds 120 --max-concurrency 1 >/dev/null
for _ in $(seq 1 150); do
  status=$("${mysql[@]}" -e "SELECT status FROM t_build_task WHERE id=$task_id")
  [[ $status == SUCCESS || $status == FAILED ]] && break
  sleep 1
done
if [[ ${status:-} != SUCCESS ]]; then
  docker logs "$agent_container" >&2 || true
  docker logs "$api_container" >&2 || true
  "${mysql[@]}" -e "SELECT id,status,builder_id,builder_attempt,error_message FROM t_build_task WHERE id=$task_id" >&2 || true
  echo "Local Agent 未完成 CUSTOMER_STORAGE 构建: ${status:-unknown}" >&2
  exit 1
fi

result=$("${mysql[@]}" -e "SELECT builder_attempt,apk_object_id,log_object_id,apk_url,log_url,apk_sha256,apk_size FROM t_build_task WHERE id=$task_id")
read -r final_attempt apk_object_id log_object_id apk_reference log_reference apk_sha apk_size <<<"$result"
[[ $final_attempt == 2 && $apk_object_id =~ ^[1-9][0-9]*$ && $log_object_id =~ ^[1-9][0-9]*$ ]] || { echo "任务恢复或产物绑定异常: $result" >&2; exit 1; }
expected_apk_ref="customer-object://$agent_id/$customer_prefix/tasks/$task_id/attempts/2/built.apk"
expected_log_ref="customer-object://$agent_id/$customer_prefix/tasks/$task_id/attempts/2/build.log"
[[ $apk_reference == "$expected_apk_ref" && $log_reference == "$expected_log_ref" ]] || { echo "任务未保存规范客户对象引用: $result" >&2; exit 1; }
output_ownership=$("${mysql[@]}" -e "SELECT COUNT(*) FROM t_storage_object WHERE id IN ($apk_object_id,$log_object_id) AND tenant_id=$tenant_id AND app_id=$app_id AND storage_mode=2 AND owner_agent_id=$agent_id AND status=3")
[[ $output_ownership == 2 ]] || { echo '客户输出对象归属或绑定状态异常' >&2; exit 1; }

docker run --rm --network "$network" -v "$temporary/result:/result" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
set -eu
mc alias set customer http://$customer_minio:9000 '$customer_root_access' '$customer_root_secret' >/dev/null
mc cp customer/$customer_bucket/$customer_prefix/tasks/$task_id/attempts/2/built.apk /result/final.apk >/dev/null
mc cp customer/$customer_bucket/$customer_prefix/tasks/$task_id/attempts/2/build.log /result/final.log >/dev/null
"
final_sha=$(shasum -a 256 "$temporary/result/final.apk" | awk '{print $1}')
[[ $final_sha == "$apk_sha" ]] || { echo '客户存储最终 APK 字节摘要与 Core 不一致' >&2; exit 1; }
docker run --rm --user 0 -v "$temporary/result:/result:ro" --entrypoint sh "$agent_image" -lc \
  "apksigner verify --verbose --print-certs /result/final.apk >/dev/null && aapt dump badging /result/final.apk | grep -q \"package: name='com.appforge.customer.acceptance'\""

control_count=$(docker run --rm --network "$network" --entrypoint /bin/sh minio/mc:RELEASE.2025-04-16T18-13-26Z -c "
mc alias set control http://minio:9000 appforge appforge_dev_minio >/dev/null
mc find control/appforge/tenants/$tenant_id --type f 2>/dev/null | wc -l
" | tr -d ' ')
[[ $control_count == 0 ]] || { echo '客户模式字节错误写入控制面对象存储' >&2; exit 1; }

for container in "$agent_container" "$api_container" appforge-core-rpc; do
  if docker logs "$container" 2>&1 | grep -Fq "$customer_secret"; then
    echo "客户存储凭据泄漏到容器日志: $container" >&2
    exit 1
  fi
done
if "${mysql[@]}" -e "SELECT CONCAT_WS('|',object_key,original_name,content_type,COALESCE(sha256,'')) FROM t_storage_object WHERE tenant_id=$tenant_id; SELECT CONCAT_WS('|',object_reference,COALESCE(sha256,'')) FROM t_hybrid_artifact_reference WHERE tenant_id=$tenant_id;" | grep -Fq "$customer_secret"; then
  echo '客户存储凭据泄漏到数据库元数据' >&2
  exit 1
fi

echo "通过: CUSTOMER_STORAGE 受限临时MinIO、合成APK/Keystore导入、mTLS登记、幂等冲突、证书/前缀拒绝、输入篡改拒绝、attempt fencing、真实构建、APK/日志回读、对象归属、控制面零字节与Secret防泄漏"
