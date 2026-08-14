#!/usr/bin/env bash

set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
go_cache=${APPFORGE_V7_GO_CACHE:-/private/tmp/appforge-go-build-cache}
go_mod_cache=${APPFORGE_V7_GO_MOD_CACHE:-/private/tmp/appforge-go-mod-cache}
vault_image=${APPFORGE_V7_VAULT_IMAGE:-hashicorp/vault:1.20}
vault_name="appforge-v7-vault-$RANDOM-$$"
vault_token="appforge-v7-acceptance-token"
temporary=$(mktemp -d)
cleanup() {
  docker rm -f "$vault_name" >/dev/null 2>&1 || true
  rm -rf "$temporary"
}
trap cleanup EXIT

umask 077
printf '%s' "$vault_token" > "$temporary/vault-token"
mkdir -p "$temporary/local"
printf '%s' '{"keystorePassword":"local-runtime-password","keyPassword":"local-runtime-key-password"}' > "$temporary/local/signing.json"

docker run -d --name "$vault_name" -e VAULT_DEV_ROOT_TOKEN_ID="$vault_token" -e VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200 \
  -p 127.0.0.1::8200 "$vault_image" >/dev/null
for _ in $(seq 1 30); do
  if docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN="$vault_token" "$vault_name" vault status >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec -e VAULT_ADDR=http://127.0.0.1:8200 -e VAULT_TOKEN="$vault_token" "$vault_name" \
  vault kv put secret/appforge keystorePassword=vault-runtime-password keyPassword=vault-runtime-key-password >/dev/null
vault_port=$(docker port "$vault_name" 8200/tcp | sed -n 's/.*://p' | tail -1)
[[ -n "$vault_port" ]] || { echo "无法解析Vault映射端口" >&2; exit 1; }

(
  cd "$repo_root/common"
  APPFORGE_VAULT_TEST_ADDRESS="http://127.0.0.1:$vault_port" \
  APPFORGE_VAULT_TEST_TOKEN_FILE="$temporary/vault-token" \
  APPFORGE_VAULT_TEST_REFERENCE="vault://secret/data/appforge" \
  GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" \
    go test ./secretprovider -run 'Test(VaultRuntimeAcceptance|FileProvider)' -count=1
)
echo "通过: 权限受限本地文件与真实Vault KV v2运行时读取，Secret未写入控制面"

if [[ -n ${APPFORGE_AWS_TEST_REGION:-} && -n ${APPFORGE_AWS_TEST_REFERENCE:-} ]]; then
  (
    cd "$repo_root/common"
    GOMODCACHE="$go_mod_cache" GOCACHE="$go_cache" go test ./secretprovider -run TestAWSSecretsManagerRuntimeAcceptance -count=1
  )
  echo "通过: 真实AWS Secrets Manager/KMS工作负载身份读取"
elif [[ ${APPFORGE_REQUIRE_AWS_ACCEPTANCE:-0} == 1 ]]; then
  echo "验收失败: 缺少APPFORGE_AWS_TEST_REGION与APPFORGE_AWS_TEST_REFERENCE" >&2
  exit 1
else
  echo "未完成: AWS真实账户验收未运行；设置测试Region、Secret引用和AWS默认凭证链后重跑"
fi

