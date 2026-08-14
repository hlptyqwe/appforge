-- AppForge V2 基础白标数据结构迁移。
-- 本文件可重复执行，并兼容已有开发数据卷。
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS sys_schema_migration (
  version VARCHAR(64) NOT NULL COMMENT '迁移版本',
  description VARCHAR(255) NOT NULL COMMENT '迁移中文说明',
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '执行时间',
  PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据库结构迁移记录';

CREATE TABLE IF NOT EXISTS t_app_branding_profile (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '品牌配置ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  profile_name VARCHAR(128) NOT NULL COMMENT '品牌配置名称',
  app_name VARCHAR(128) NOT NULL COMMENT '构建后展示的应用名称',
  logo_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '品牌Logo存储对象ID',
  splash_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '品牌启动图存储对象ID',
  api_host VARCHAR(500) NOT NULL COMMENT '构建后应用使用的API地址',
  rewrite_mode TINYINT NOT NULL DEFAULT 1 COMMENT '重写模式：1资源重建 2运行时契约',
  launcher_icon_target VARCHAR(255) DEFAULT NULL COMMENT 'Launcher图标资源目标，例如mipmap/ic_launcher',
  splash_resource_target VARCHAR(255) DEFAULT NULL COMMENT '启动图资源目标，例如drawable/splash_logo',
  runtime_config JSON DEFAULT NULL COMMENT '运行时契约扩展配置JSON',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1草稿 2启用 3停用',
  revision INT NOT NULL DEFAULT 1 COMMENT '品牌配置修订号，修改时单调递增',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_branding_tenant_app_name (tenant_id, app_id, profile_name) COMMENT '同一应用品牌配置名称唯一',
  KEY idx_branding_tenant_app_status (tenant_id, app_id, status) COMMENT '租户应用品牌状态查询索引',
  KEY idx_branding_logo_object (logo_object_id) COMMENT '品牌Logo对象查询索引',
  KEY idx_branding_splash_object (splash_object_id) COMMENT '品牌启动图对象查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='应用品牌白标配置';

CREATE TABLE IF NOT EXISTS t_branding_preflight (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '品牌兼容性预检ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  branding_profile_id BIGINT NOT NULL COMMENT '品牌配置ID',
  branding_revision INT NOT NULL COMMENT '预检时的品牌配置修订号',
  version_id BIGINT NOT NULL COMMENT '被检查的应用版本ID',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1待处理 2兼容 3不兼容 4执行失败',
  report_json JSON DEFAULT NULL COMMENT '结构化预检报告JSON',
  source_apk_sha256 CHAR(64) DEFAULT NULL COMMENT '预检源APK的SHA-256快照',
  toolchain_version VARCHAR(128) DEFAULT NULL COMMENT '执行预检的工具链版本',
  builder_id VARCHAR(128) DEFAULT NULL COMMENT '执行预检的Builder节点ID',
  builder_attempt INT NOT NULL DEFAULT 0 COMMENT '预检领取代次，用于阻止过期Worker回写',
  start_time DATETIME DEFAULT NULL COMMENT '预检开始时间',
  finish_time DATETIME DEFAULT NULL COMMENT '预检完成时间',
  lease_until DATETIME DEFAULT NULL COMMENT 'Builder预检租约到期时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_preflight_profile_revision_version (tenant_id, branding_profile_id, branding_revision, version_id) COMMENT '同一品牌修订和应用版本预检唯一',
  KEY idx_preflight_tenant_app_status (tenant_id, app_id, status, create_time) COMMENT '租户应用预检状态查询索引',
  KEY idx_preflight_version (version_id, create_time) COMMENT '应用版本预检查询索引',
  KEY idx_preflight_queue (status, lease_until, id) COMMENT '品牌预检领取队列索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='品牌白标兼容性预检记录';

DROP PROCEDURE IF EXISTS appforge_apply_v2_branding;

DELIMITER //
CREATE PROCEDURE appforge_apply_v2_branding()
BEGIN
  ALTER TABLE t_storage_object
    MODIFY COLUMN object_type TINYINT NOT NULL COMMENT '对象类型：1原始APK 2签名文件 3构建APK 4构建日志 5品牌Logo 6品牌启动图';

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_branding_preflight' AND COLUMN_NAME = 'builder_id'
  ) THEN
    ALTER TABLE t_branding_preflight
      ADD COLUMN builder_id VARCHAR(128) DEFAULT NULL COMMENT '执行预检的Builder节点ID' AFTER toolchain_version,
      ADD COLUMN builder_attempt INT NOT NULL DEFAULT 0 COMMENT '预检领取代次，用于阻止过期Worker回写' AFTER builder_id,
      ADD COLUMN lease_until DATETIME DEFAULT NULL COMMENT 'Builder预检租约到期时间' AFTER finish_time;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_branding_preflight' AND INDEX_NAME = 'idx_preflight_queue'
  ) THEN
    ALTER TABLE t_branding_preflight
      ADD KEY idx_preflight_queue (status, lease_until, id) COMMENT '品牌预检领取队列索引';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'branding_profile_id'
  ) THEN
    ALTER TABLE t_build_task
      ADD COLUMN branding_profile_id BIGINT NOT NULL DEFAULT 0 COMMENT '构建时使用的品牌配置ID，0表示不启用白标' AFTER build_config;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'branding_revision'
  ) THEN
    ALTER TABLE t_build_task
      ADD COLUMN branding_revision INT NOT NULL DEFAULT 0 COMMENT '构建时使用的品牌配置修订号' AFTER branding_profile_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'branding_snapshot'
  ) THEN
    ALTER TABLE t_build_task
      ADD COLUMN branding_snapshot JSON DEFAULT NULL COMMENT '构建时固化的品牌配置快照JSON' AFTER branding_revision;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND INDEX_NAME = 'idx_build_branding_profile'
  ) THEN
    ALTER TABLE t_build_task
      ADD KEY idx_build_branding_profile (branding_profile_id, branding_revision) COMMENT '品牌配置构建任务查询索引';
  END IF;
END//
DELIMITER ;

CALL appforge_apply_v2_branding();
DROP PROCEDURE IF EXISTS appforge_apply_v2_branding;

INSERT INTO sys_menu (
  id, parent_id, app_scope, name, menu_type, method, path, component, perms,
  icon, sort, visible, enabled, create_times, update_times
) VALUES
  (16, 1, 1, '品牌配置', 2, '', '/platform/branding-profiles', 'platform/branding-profiles', '', 'PictureFilled', 45, 1, 1, 0, 0),
  (1071, 16, 1, '查看品牌配置列表', 3, 'GET', '/core/branding-profiles', '', 'core:branding:view', '', 1, 2, 1, 0, 0),
  (1072, 16, 1, '查看品牌配置详情', 3, 'GET', '/core/branding-profiles/:id', '', 'core:branding:view', '', 2, 2, 1, 0, 0),
  (1073, 16, 1, '创建品牌配置', 3, 'POST', '/core/branding-profiles', '', 'core:branding:add', '', 3, 2, 1, 0, 0),
  (1074, 16, 1, '修改品牌配置', 3, 'PUT', '/core/branding-profiles/:id', '', 'core:branding:update', '', 4, 2, 1, 0, 0),
  (1075, 16, 1, '修改品牌配置状态', 3, 'POST', '/core/branding-profiles/:id/status', '', 'core:branding:status', '', 5, 2, 1, 0, 0),
  (1076, 16, 1, '创建品牌兼容性预检', 3, 'POST', '/core/branding-profiles/:id/preflight', '', 'core:branding:preflight', '', 6, 2, 1, 0, 0),
  (1077, 16, 1, '查看品牌预检记录', 3, 'GET', '/core/branding-preflights', '', 'core:branding:view', '', 7, 2, 1, 0, 0),
  (1078, 16, 1, '查看品牌预检详情', 3, 'GET', '/core/branding-preflights/:id', '', 'core:branding:view', '', 8, 2, 1, 0, 0),

  (3070, 3000, 2, '品牌配置', 2, '', '/platform/branding-profiles', 'platform/branding-profiles', '', 'PictureFilled', 45, 1, 1, 0, 0),
  (3261, 3070, 2, '查看品牌配置列表', 3, 'GET', '/core/branding-profiles', '', 'core:branding:view', '', 1, 2, 1, 0, 0),
  (3262, 3070, 2, '查看品牌配置详情', 3, 'GET', '/core/branding-profiles/:id', '', 'core:branding:view', '', 2, 2, 1, 0, 0),
  (3263, 3070, 2, '创建品牌配置', 3, 'POST', '/core/branding-profiles', '', 'core:branding:add', '', 3, 2, 1, 0, 0),
  (3264, 3070, 2, '修改品牌配置', 3, 'PUT', '/core/branding-profiles/:id', '', 'core:branding:update', '', 4, 2, 1, 0, 0),
  (3265, 3070, 2, '修改品牌配置状态', 3, 'POST', '/core/branding-profiles/:id/status', '', 'core:branding:status', '', 5, 2, 1, 0, 0),
  (3266, 3070, 2, '创建品牌兼容性预检', 3, 'POST', '/core/branding-profiles/:id/preflight', '', 'core:branding:preflight', '', 6, 2, 1, 0, 0),
  (3267, 3070, 2, '查看品牌预检记录', 3, 'GET', '/core/branding-preflights', '', 'core:branding:view', '', 7, 2, 1, 0, 0),
  (3268, 3070, 2, '查看品牌预检详情', 3, 'GET', '/core/branding-preflights/:id', '', 'core:branding:view', '', 8, 2, 1, 0, 0)
ON DUPLICATE KEY UPDATE
  name = VALUES(name), parent_id = VALUES(parent_id), app_scope = VALUES(app_scope), method = VALUES(method),
  path = VALUES(path), component = VALUES(component), perms = VALUES(perms), icon = VALUES(icon),
  sort = VALUES(sort), visible = VALUES(visible), enabled = VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.app_scope = r.app_scope
WHERE r.code IN ('owner', 'admin')
  AND m.id IN (16, 1071, 1072, 1073, 1074, 1075, 1076, 1077, 1078,
               3070, 3261, 3262, 3263, 3264, 3265, 3266, 3267, 3268);

INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.app_scope = r.app_scope
WHERE r.code = 'viewer'
  AND m.id IN (16, 1071, 1072, 1077, 1078);

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260814_70_v2_branding', 'V2基础白标配置、兼容性预检与构建快照')
ON DUPLICATE KEY UPDATE description = VALUES(description);
