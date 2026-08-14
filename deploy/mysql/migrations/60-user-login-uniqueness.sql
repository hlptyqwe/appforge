-- AppForge V1 登录账号唯一性约束。
-- 登录接口以 app_scope + username 定位账号，因此该组合必须全局唯一，不能仅在租户内唯一。
SET NAMES utf8mb4;

SET @has_scope_username_index = (
  SELECT COUNT(*)
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'sys_user'
    AND index_name = 'uk_scope_username'
);
SET @add_scope_username_index_sql = IF(
  @has_scope_username_index = 0,
  'ALTER TABLE sys_user ADD UNIQUE KEY uk_scope_username (app_scope, username) COMMENT ''同一应用端登录账号全局唯一，避免跨租户登录歧义''',
  'SELECT 1'
);
PREPARE add_scope_username_index_stmt FROM @add_scope_username_index_sql;
EXECUTE add_scope_username_index_stmt;
DEALLOCATE PREPARE add_scope_username_index_stmt;

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260813_60_user_login_uniqueness', 'V1同一应用端用户名全局唯一约束')
ON DUPLICATE KEY UPDATE description = VALUES(description);
