#!/bin/sh

set -eu

expected=""
for argument in "$@"; do
  case "$argument" in
    --expected=*) expected=${argument#--expected=} ;;
    *) echo "不支持的迁移参数: $argument" >&2; exit 2 ;;
  esac
done

: "${MYSQL_HOST:?required}"
: "${MYSQL_DATABASE:?required}"
: "${MYSQL_USER:?required}"
: "${MYSQL_PASSWORD:?required}"
: "${APPFORGE_BOOTSTRAP_ADMIN_USERNAME:?required}"
: "${APPFORGE_BOOTSTRAP_PASSWORD_BCRYPT_BASE64:?required}"

export MYSQL_PWD=$MYSQL_PASSWORD
mysql_cmd="mysql --protocol=TCP -h $MYSQL_HOST -P ${MYSQL_PORT:-3306} -u $MYSQL_USER --default-character-set=utf8mb4 $MYSQL_DATABASE"

until mysqladmin --protocol=TCP -h "$MYSQL_HOST" -P "${MYSQL_PORT:-3306}" -u "$MYSQL_USER" ping --silent; do
  sleep 2
done

lock=$($mysql_cmd -N -e "SELECT GET_LOCK('appforge-schema-migration',60)")
[ "$lock" = "1" ] || { echo "无法获取数据库迁移锁" >&2; exit 1; }
trap '$mysql_cmd -N -e "SELECT RELEASE_LOCK('\''appforge-schema-migration'\'')" >/dev/null 2>&1 || true' EXIT

table_count=$($mysql_cmd -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND (table_name LIKE 'sys\\_%' OR table_name LIKE 't\\_%')")
migration_table=$($mysql_cmd -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='sys_schema_migration'")

if [ "$table_count" = "0" ]; then
  $mysql_cmd < /bootstrap/system.sql
  $mysql_cmd < /bootstrap/core.sql
  $mysql_cmd < /bootstrap/seed.sql

  bootstrap_hash=$(printf '%s' "$APPFORGE_BOOTSTRAP_PASSWORD_BCRYPT_BASE64" | base64 -d)
  case "$bootstrap_hash" in
    \$2a\$*|\$2b\$*|\$2y\$*) ;;
    *) echo "启动管理员密码必须是 Base64 编码的 bcrypt 哈希" >&2; exit 1 ;;
  esac
  case "$APPFORGE_BOOTSTRAP_ADMIN_USERNAME" in
    *[!A-Za-z0-9._-]*|'') echo "启动管理员账号包含非法字符" >&2; exit 1 ;;
  esac
  $mysql_cmd -e "UPDATE sys_user SET username='${APPFORGE_BOOTSTRAP_ADMIN_USERNAME}',password='${bootstrap_hash}',nickname='Enterprise Owner' WHERE id=1 AND app_scope=1; UPDATE sys_tenant_domain SET origin='${APPFORGE_PUBLIC_ORIGIN}' WHERE id=1; UPDATE sys_config SET config_value=JSON_SET(config_value,'$.oss_domain','${APPFORGE_PUBLIC_ORIGIN}/appforge','$.minio.access_key_id','${APPFORGE_MINIO_ACCESS_KEY}','$.minio.access_key_secret','${APPFORGE_MINIO_SECRET_KEY}','$.minio.bucket_url','${APPFORGE_PUBLIC_ORIGIN}/appforge'),remark='生产私有对象存储配置' WHERE tenant_id=0 AND config_key='OBJECT_STORAGE';"
elif [ "$migration_table" = "0" ]; then
  echo "检测到已有 AppForge 表但缺少 sys_schema_migration；为避免破坏数据，拒绝自动初始化" >&2
  exit 1
fi

for file in $(find /migrations -maxdepth 1 -type f -name '*.sql' | sort -V); do
  echo "应用迁移 $(basename "$file")"
  $mysql_cmd < "$file"
done

if [ -n "$expected" ]; then
  present=$($mysql_cmd -N -e "SELECT COUNT(*) FROM sys_schema_migration WHERE version='${expected}'")
  [ "$present" = "1" ] || { echo "未达到期望 Schema 版本: $expected" >&2; exit 1; }
fi

echo "AppForge 数据库迁移完成"
