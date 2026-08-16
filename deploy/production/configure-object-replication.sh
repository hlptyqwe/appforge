#!/usr/bin/env bash

set -euo pipefail

delivery_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
set -a
# shellcheck disable=SC1091
source "${APPFORGE_ENV_FILE:-$delivery_dir/.env}"
set +a

endpoint=${APPFORGE_REPLICA_ENDPOINT:?required}
access_key=${APPFORGE_REPLICA_ACCESS_KEY:?required}
secret_key=${APPFORGE_REPLICA_SECRET_KEY:?required}
bucket=${APPFORGE_REPLICA_BUCKET:-appforge}
rule_id=${APPFORGE_REPLICA_RULE_ID:-appforge-dr}
sync_mode=${APPFORGE_REPLICA_SYNC:-false}
allow_insecure=${APPFORGE_ALLOW_INSECURE_REPLICA:-false}

if [[ $allow_insecure == true ]]; then
  [[ $endpoint =~ ^https?://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || { echo "副本 Endpoint 格式无效" >&2; exit 1; }
else
  [[ $endpoint =~ ^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$ ]] || { echo "副本 Endpoint 必须使用 HTTPS" >&2; exit 1; }
fi
[[ $access_key =~ ^[A-Za-z0-9._~+/=-]+$ && $secret_key =~ ^[A-Za-z0-9._~+/=-]+$ ]] || { echo "副本凭据包含不安全字符" >&2; exit 1; }
[[ $bucket =~ ^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$ ]] || { echo "副本桶名无效" >&2; exit 1; }
[[ $rule_id =~ ^[A-Za-z0-9._-]{1,64}$ ]] || { echo "复制规则 ID 无效" >&2; exit 1; }
[[ $sync_mode == false || $sync_mode == true ]] || { echo "APPFORGE_REPLICA_SYNC 只能为 true 或 false" >&2; exit 1; }

docker compose -f "$delivery_dir/docker-compose.yml" run --rm --no-deps --entrypoint /bin/sh minio-init -c '
  set -eu
  mc alias set source http://minio:9000 "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null
  mc alias set replica "$REPLICA_ENDPOINT" "$REPLICA_ACCESS_KEY" "$REPLICA_SECRET_KEY" >/dev/null
  mc mb --ignore-existing "source/$REPLICA_BUCKET" >/dev/null
  mc mb --ignore-existing "replica/$REPLICA_BUCKET" >/dev/null
  mc version enable "source/$REPLICA_BUCKET" >/dev/null
  mc version enable "replica/$REPLICA_BUCKET" >/dev/null
  mc replicate remove --id "$REPLICA_RULE_ID" "source/$REPLICA_BUCKET" >/dev/null 2>&1 || true
  set -- "source/$REPLICA_BUCKET" --remote-bucket "replica/$REPLICA_BUCKET" --priority 1 --id "$REPLICA_RULE_ID" --replicate existing-objects,delete,delete-marker,metadata-sync
  if [ "$REPLICA_SYNC" = true ]; then set -- "$@" --sync; fi
  mc replicate add "$@" >/dev/null
  mc replicate list "source/$REPLICA_BUCKET"
'
echo "对象存储复制规则已配置: bucket=$bucket rule=$rule_id sync=$sync_mode"
