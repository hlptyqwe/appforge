-- AppForge V1 文件上传与对象关联迁移。
-- 本文件可重复执行，并兼容已有开发数据卷。

CREATE TABLE IF NOT EXISTS sys_schema_migration (
  version VARCHAR(64) NOT NULL COMMENT '迁移版本',
  description VARCHAR(255) NOT NULL COMMENT '迁移中文说明',
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '执行时间',
  PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库结构迁移记录';

CREATE TABLE IF NOT EXISTS t_storage_object (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '存储对象ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联应用ID，上传时允许为0',
  object_type TINYINT NOT NULL COMMENT '对象类型：1原始APK 2签名文件 3构建APK 4构建日志',
  object_key VARCHAR(500) NOT NULL COMMENT '私有对象存储Key',
  original_name VARCHAR(255) NOT NULL COMMENT '用户上传时的原始文件名',
  content_type VARCHAR(128) NOT NULL DEFAULT 'application/octet-stream' COMMENT '对象内容类型',
  size_bytes BIGINT NOT NULL DEFAULT 0 COMMENT '对象大小，单位字节',
  sha256 CHAR(64) DEFAULT NULL COMMENT '对象SHA-256，小写十六进制',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1上传中 2已就绪 3已绑定 4已删除 5失败',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_storage_object_key (object_key) COMMENT '对象Key唯一索引',
  KEY idx_storage_tenant_type_status (tenant_id, object_type, status) COMMENT '租户对象类型状态查询索引',
  KEY idx_storage_tenant_app (tenant_id, app_id) COMMENT '租户应用对象查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='私有对象存储元数据';

DROP PROCEDURE IF EXISTS appforge_apply_v1_storage;

DELIMITER //
CREATE PROCEDURE appforge_apply_v1_storage()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_app_version' AND COLUMN_NAME = 'source_apk_object_id'
  ) THEN
    ALTER TABLE t_app_version
      ADD COLUMN source_apk_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '原始APK存储对象ID' AFTER version_name;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_app_version' AND INDEX_NAME = 'idx_version_source_object'
  ) THEN
    ALTER TABLE t_app_version
      ADD KEY idx_version_source_object (source_apk_object_id) COMMENT '原始APK对象查询索引';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_app_signing_config' AND COLUMN_NAME = 'keystore_object_id'
  ) THEN
    ALTER TABLE t_app_signing_config
      ADD COLUMN keystore_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '签名文件存储对象ID' AFTER name;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_app_signing_config' AND INDEX_NAME = 'idx_signing_keystore_object'
  ) THEN
    ALTER TABLE t_app_signing_config
      ADD KEY idx_signing_keystore_object (keystore_object_id) COMMENT '签名文件对象查询索引';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'source_apk_object_id'
  ) THEN
    ALTER TABLE t_build_task
      ADD COLUMN source_apk_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '构建时使用的原始APK对象ID快照' AFTER version_name;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'apk_object_id'
  ) THEN
    ALTER TABLE t_build_task
      ADD COLUMN apk_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '构建产物APK存储对象ID' AFTER priority;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'log_object_id'
  ) THEN
    ALTER TABLE t_build_task
      ADD COLUMN log_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '构建日志存储对象ID' AFTER apk_size;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND INDEX_NAME = 'idx_build_source_object'
  ) THEN
    ALTER TABLE t_build_task
      ADD KEY idx_build_source_object (source_apk_object_id) COMMENT '构建源APK对象查询索引';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND INDEX_NAME = 'idx_build_apk_object'
  ) THEN
    ALTER TABLE t_build_task
      ADD KEY idx_build_apk_object (apk_object_id) COMMENT '构建产物对象查询索引';
  END IF;
END//
DELIMITER ;

CALL appforge_apply_v1_storage();
DROP PROCEDURE IF EXISTS appforge_apply_v1_storage;

INSERT INTO sys_menu (
  id, parent_id, app_scope, name, menu_type, method, path, component, perms,
  icon, sort, visible, enabled, create_times, update_times
) VALUES
  (1061, 11, 1, '初始化业务文件上传', 3, 'POST', '/core/uploads/initiate', '', 'core:storage:upload', '', 10, 2, 1, 0, 0),
  (1062, 11, 1, '完成业务文件上传', 3, 'POST', '/core/uploads/:id/complete', '', 'core:storage:upload', '', 11, 2, 1, 0, 0),
  (1063, 14, 1, '下载业务文件', 3, 'GET', '/core/storage/objects/:id/download', '', 'core:storage:download', '', 12, 2, 1, 0, 0)
ON DUPLICATE KEY UPDATE
  name = VALUES(name), method = VALUES(method), path = VALUES(path), perms = VALUES(perms),
  parent_id = VALUES(parent_id), app_scope = VALUES(app_scope), enabled = VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.id IN (1061, 1062, 1063)
WHERE r.code IN ('owner', 'admin');

UPDATE sys_config
SET config_value = JSON_SET(config_value, '$.minio.endpoint', 'http://minio:9000')
WHERE tenant_id = 0
  AND config_key = 'OBJECT_STORAGE'
  AND JSON_UNQUOTE(JSON_EXTRACT(config_value, '$.minio.endpoint')) = 'minio:9000';

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260813_40_v1_storage', 'V1私有对象上传与业务对象关联')
ON DUPLICATE KEY UPDATE description = VALUES(description);
