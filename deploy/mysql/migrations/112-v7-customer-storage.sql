-- AppForge V7 CUSTOMER_STORAGE 显式对象归属字段。
-- 本文件可重复执行；历史对象统一归为控制面存储。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

SET @storage_mode_exists = (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='t_storage_object' AND column_name='storage_mode'
);
SET @storage_mode_sql = IF(
  @storage_mode_exists=0,
  'ALTER TABLE t_storage_object ADD COLUMN storage_mode TINYINT NOT NULL DEFAULT 1 COMMENT ''存储模式：1控制面存储 2客户存储'' AFTER status',
  'SELECT 1'
);
PREPARE storage_mode_stmt FROM @storage_mode_sql;
EXECUTE storage_mode_stmt;
DEALLOCATE PREPARE storage_mode_stmt;

SET @owner_agent_exists = (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='t_storage_object' AND column_name='owner_agent_id'
);
SET @owner_agent_sql = IF(
  @owner_agent_exists=0,
  'ALTER TABLE t_storage_object ADD COLUMN owner_agent_id BIGINT NOT NULL DEFAULT 0 COMMENT ''客户存储所属Local Agent ID，控制面存储为0'' AFTER storage_mode',
  'SELECT 1'
);
PREPARE owner_agent_stmt FROM @owner_agent_sql;
EXECUTE owner_agent_stmt;
DEALLOCATE PREPARE owner_agent_stmt;

SET @storage_agent_index_exists = (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema=DATABASE() AND table_name='t_storage_object' AND index_name='idx_storage_mode_agent'
);
SET @storage_agent_index_sql = IF(
  @storage_agent_index_exists=0,
  'ALTER TABLE t_storage_object ADD KEY idx_storage_mode_agent (tenant_id,storage_mode,owner_agent_id,status) COMMENT ''客户存储对象归属和状态查询索引''',
  'SELECT 1'
);
PREPARE storage_agent_index_stmt FROM @storage_agent_index_sql;
EXECUTE storage_agent_index_stmt;
DEALLOCATE PREPARE storage_agent_index_stmt;

UPDATE t_storage_object
SET storage_mode=1,owner_agent_id=0
WHERE storage_mode IS NULL OR storage_mode NOT IN (1,2);

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260815_112_v7_customer_storage','V7 CUSTOMER_STORAGE对象归属和Local Agent绑定字段')
ON DUPLICATE KEY UPDATE description=VALUES(description);
