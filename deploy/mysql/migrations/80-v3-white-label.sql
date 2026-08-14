-- AppForge V3 高级白标、模板修订和包名证书绑定迁移。
-- 本文件可重复执行，并兼容已有开发数据卷。
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS t_white_label_template (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '白标模板ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  template_code VARCHAR(64) NOT NULL COMMENT '模板编码',
  template_name VARCHAR(128) NOT NULL COMMENT '模板名称',
  source_version_id BIGINT NOT NULL COMMENT '模板源应用版本ID',
  parameter_schema JSON DEFAULT NULL COMMENT '模板参数JSON Schema',
  compatibility_rules JSON DEFAULT NULL COMMENT '模板兼容性规则JSON',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1草稿 2已发布 3停用',
  published_revision INT NOT NULL DEFAULT 0 COMMENT '当前已发布修订号，0表示未发布',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_white_template_tenant_code (tenant_id, template_code) COMMENT '租户模板编码唯一',
  KEY idx_white_template_tenant_app_status (tenant_id, app_id, status) COMMENT '租户应用模板状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V3白标模板';

CREATE TABLE IF NOT EXISTS t_white_label_template_revision (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '白标模板修订ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  template_id BIGINT NOT NULL COMMENT '白标模板ID',
  revision INT NOT NULL COMMENT '模板修订号',
  package_name_rule JSON NOT NULL COMMENT '动态包名规则JSON',
  manifest_patch JSON DEFAULT NULL COMMENT 'Manifest声明式补丁JSON',
  resource_patch JSON DEFAULT NULL COMMENT '资源声明式补丁JSON',
  extension_files JSON DEFAULT NULL COMMENT '受控扩展文件声明JSON',
  expected_artifacts JSON DEFAULT NULL COMMENT '产物验收规则JSON',
  checksum CHAR(64) NOT NULL COMMENT '规范化修订内容SHA-256',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1草稿 2已发布 3已取代',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_white_revision_template_revision (template_id, revision) COMMENT '模板修订号唯一',
  UNIQUE KEY uk_white_revision_template_checksum (template_id, checksum) COMMENT '模板修订内容唯一',
  KEY idx_white_revision_tenant_status (tenant_id, status, create_time) COMMENT '租户修订状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V3白标模板不可变修订';

CREATE TABLE IF NOT EXISTS t_white_label_product (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '白标产品ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  product_code VARCHAR(64) NOT NULL COMMENT '白标产品编码',
  product_name VARCHAR(128) NOT NULL COMMENT '白标产品名称',
  template_id BIGINT NOT NULL COMMENT '白标模板ID',
  template_revision INT NOT NULL COMMENT '绑定的已发布模板修订号',
  branding_profile_id BIGINT NOT NULL COMMENT '品牌配置ID',
  package_name VARCHAR(255) NOT NULL COMMENT '白标产品Android applicationId',
  signing_config_id BIGINT NOT NULL COMMENT '独立签名配置ID',
  parameter_values JSON DEFAULT NULL COMMENT '模板参数值JSON',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1草稿 2启用 3停用',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_white_product_tenant_code (tenant_id, product_code) COMMENT '租户产品编码唯一',
  UNIQUE KEY uk_white_product_tenant_package (tenant_id, package_name) COMMENT '租户白标包名唯一',
  KEY idx_white_product_tenant_app_status (tenant_id, app_id, status) COMMENT '租户应用白标产品状态查询索引',
  KEY idx_white_product_template (template_id, template_revision) COMMENT '模板修订产品查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V3白标产品';

CREATE TABLE IF NOT EXISTS t_package_certificate_binding (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '包名证书绑定ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  package_name VARCHAR(255) NOT NULL COMMENT 'Android applicationId',
  certificate_sha256 CHAR(64) NOT NULL COMMENT '签名证书SHA-256指纹',
  signing_config_id BIGINT NOT NULL COMMENT '签名配置ID',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1生效 2变更待审批 3已废弃',
  first_build_task_id BIGINT NOT NULL DEFAULT 0 COMMENT '首次使用构建任务ID',
  last_build_task_id BIGINT NOT NULL DEFAULT 0 COMMENT '最近使用构建任务ID',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_package_certificate_tenant_package (tenant_id, package_name) COMMENT '租户包名当前证书唯一',
  KEY idx_package_certificate_signing (signing_config_id, status) COMMENT '签名配置绑定查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V3包名与签名证书历史绑定';

DROP PROCEDURE IF EXISTS appforge_apply_v3_white_label;
DELIMITER //
CREATE PROCEDURE appforge_apply_v3_white_label()
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_app_signing_config' AND COLUMN_NAME = 'certificate_sha256'
  ) THEN
    ALTER TABLE t_app_signing_config
      ADD COLUMN certificate_sha256 CHAR(64) DEFAULT NULL COMMENT '签名证书SHA-256指纹，小写十六进制' AFTER key_alias;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'white_label_product_id'
  ) THEN
    ALTER TABLE t_build_task
      ADD COLUMN white_label_product_id BIGINT NOT NULL DEFAULT 0 COMMENT 'V3白标产品ID，0表示非高级白标构建' AFTER branding_snapshot,
      ADD COLUMN template_revision INT NOT NULL DEFAULT 0 COMMENT 'V3构建使用的模板修订号' AFTER white_label_product_id,
      ADD COLUMN template_snapshot JSON DEFAULT NULL COMMENT 'V3模板和产品不可变构建快照JSON' AFTER template_revision;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND INDEX_NAME = 'idx_build_white_label_product'
  ) THEN
    ALTER TABLE t_build_task
      ADD KEY idx_build_white_label_product (white_label_product_id, template_revision) COMMENT '白标产品构建任务查询索引';
  END IF;
END//
DELIMITER ;

CALL appforge_apply_v3_white_label();
DROP PROCEDURE IF EXISTS appforge_apply_v3_white_label;

INSERT INTO sys_menu (
  id, parent_id, app_scope, name, menu_type, method, path, component, perms,
  icon, sort, visible, enabled, create_times, update_times
) VALUES
  (17, 1, 1, '白标模板', 2, '', '/platform/white-label-templates', 'platform/white-label-templates', '', 'DocumentCopy', 46, 1, 1, 0, 0),
  (1081, 17, 1, '查看白标模板列表', 3, 'GET', '/core/white-label/templates', '', 'core:white-label-template:view', '', 1, 2, 1, 0, 0),
  (1082, 17, 1, '查看白标模板详情', 3, 'GET', '/core/white-label/templates/:id', '', 'core:white-label-template:view', '', 2, 2, 1, 0, 0),
  (1083, 17, 1, '创建白标模板', 3, 'POST', '/core/white-label/templates', '', 'core:white-label-template:add', '', 3, 2, 1, 0, 0),
  (1084, 17, 1, '创建模板修订', 3, 'POST', '/core/white-label/templates/:id/revisions', '', 'core:white-label-template:revision', '', 4, 2, 1, 0, 0),
  (1085, 17, 1, '查看模板修订', 3, 'GET', '/core/white-label/templates/:id/revisions', '', 'core:white-label-template:view', '', 5, 2, 1, 0, 0),
  (1086, 17, 1, '发布模板修订', 3, 'POST', '/core/white-label/templates/:id/publish', '', 'core:white-label-template:publish', '', 6, 2, 1, 0, 0),
	(1087, 17, 1, '修改白标模板状态', 3, 'POST', '/core/white-label/templates/:id/status', '', 'core:white-label-template:status', '', 7, 2, 1, 0, 0),
  (1101, 17, 1, '修改白标模板', 3, 'PUT', '/core/white-label/templates/:id', '', 'core:white-label-template:update', '', 8, 2, 1, 0, 0),
  (1102, 17, 1, '复制白标模板', 3, 'POST', '/core/white-label/templates/:id/copy', '', 'core:white-label-template:copy', '', 9, 2, 1, 0, 0),
  (1103, 17, 1, '删除白标模板', 3, 'DELETE', '/core/white-label/templates/:id', '', 'core:white-label-template:delete', '', 10, 2, 1, 0, 0),
  (1104, 17, 1, '查看模板修订详情', 3, 'GET', '/core/white-label/templates/:id/revisions/:revision', '', 'core:white-label-template:view', '', 11, 2, 1, 0, 0),
  (1105, 17, 1, '修改模板草稿修订', 3, 'PUT', '/core/white-label/templates/:id/revisions/:revision', '', 'core:white-label-template:revision', '', 12, 2, 1, 0, 0),
  (1106, 17, 1, '删除模板草稿修订', 3, 'DELETE', '/core/white-label/templates/:id/revisions/:revision', '', 'core:white-label-template:revision', '', 13, 2, 1, 0, 0),
  (18, 1, 1, '白标产品', 2, '', '/platform/white-label-products', 'platform/white-label-products', '', 'GoodsFilled', 47, 1, 1, 0, 0),
  (1091, 18, 1, '查看白标产品列表', 3, 'GET', '/core/white-label/products', '', 'core:white-label-product:view', '', 1, 2, 1, 0, 0),
  (1092, 18, 1, '查看白标产品详情', 3, 'GET', '/core/white-label/products/:id', '', 'core:white-label-product:view', '', 2, 2, 1, 0, 0),
  (1093, 18, 1, '创建白标产品', 3, 'POST', '/core/white-label/products', '', 'core:white-label-product:add', '', 3, 2, 1, 0, 0),
  (1094, 18, 1, '修改白标产品', 3, 'PUT', '/core/white-label/products/:id', '', 'core:white-label-product:update', '', 4, 2, 1, 0, 0),
  (1095, 18, 1, '修改白标产品状态', 3, 'POST', '/core/white-label/products/:id/status', '', 'core:white-label-product:status', '', 5, 2, 1, 0, 0),
  (1096, 18, 1, '预检白标产品', 3, 'POST', '/core/white-label/products/:id/preflight', '', 'core:white-label-product:preflight', '', 6, 2, 1, 0, 0),
  (1097, 18, 1, '删除白标产品', 3, 'DELETE', '/core/white-label/products/:id', '', 'core:white-label-product:delete', '', 7, 2, 1, 0, 0),

  (3080, 3000, 2, '白标模板', 2, '', '/platform/white-label-templates', 'platform/white-label-templates', '', 'DocumentCopy', 46, 1, 1, 0, 0),
  (3271, 3080, 2, '查看白标模板列表', 3, 'GET', '/core/white-label/templates', '', 'core:white-label-template:view', '', 1, 2, 1, 0, 0),
  (3272, 3080, 2, '查看白标模板详情', 3, 'GET', '/core/white-label/templates/:id', '', 'core:white-label-template:view', '', 2, 2, 1, 0, 0),
  (3273, 3080, 2, '创建白标模板', 3, 'POST', '/core/white-label/templates', '', 'core:white-label-template:add', '', 3, 2, 1, 0, 0),
  (3274, 3080, 2, '创建模板修订', 3, 'POST', '/core/white-label/templates/:id/revisions', '', 'core:white-label-template:revision', '', 4, 2, 1, 0, 0),
  (3275, 3080, 2, '查看模板修订', 3, 'GET', '/core/white-label/templates/:id/revisions', '', 'core:white-label-template:view', '', 5, 2, 1, 0, 0),
  (3276, 3080, 2, '发布模板修订', 3, 'POST', '/core/white-label/templates/:id/publish', '', 'core:white-label-template:publish', '', 6, 2, 1, 0, 0),
	(3277, 3080, 2, '修改白标模板状态', 3, 'POST', '/core/white-label/templates/:id/status', '', 'core:white-label-template:status', '', 7, 2, 1, 0, 0),
  (3291, 3080, 2, '修改白标模板', 3, 'PUT', '/core/white-label/templates/:id', '', 'core:white-label-template:update', '', 8, 2, 1, 0, 0),
  (3292, 3080, 2, '复制白标模板', 3, 'POST', '/core/white-label/templates/:id/copy', '', 'core:white-label-template:copy', '', 9, 2, 1, 0, 0),
  (3293, 3080, 2, '删除白标模板', 3, 'DELETE', '/core/white-label/templates/:id', '', 'core:white-label-template:delete', '', 10, 2, 1, 0, 0),
  (3294, 3080, 2, '查看模板修订详情', 3, 'GET', '/core/white-label/templates/:id/revisions/:revision', '', 'core:white-label-template:view', '', 11, 2, 1, 0, 0),
  (3295, 3080, 2, '修改模板草稿修订', 3, 'PUT', '/core/white-label/templates/:id/revisions/:revision', '', 'core:white-label-template:revision', '', 12, 2, 1, 0, 0),
  (3296, 3080, 2, '删除模板草稿修订', 3, 'DELETE', '/core/white-label/templates/:id/revisions/:revision', '', 'core:white-label-template:revision', '', 13, 2, 1, 0, 0),
  (3090, 3000, 2, '白标产品', 2, '', '/platform/white-label-products', 'platform/white-label-products', '', 'GoodsFilled', 47, 1, 1, 0, 0),
  (3281, 3090, 2, '查看白标产品列表', 3, 'GET', '/core/white-label/products', '', 'core:white-label-product:view', '', 1, 2, 1, 0, 0),
  (3282, 3090, 2, '查看白标产品详情', 3, 'GET', '/core/white-label/products/:id', '', 'core:white-label-product:view', '', 2, 2, 1, 0, 0),
  (3283, 3090, 2, '创建白标产品', 3, 'POST', '/core/white-label/products', '', 'core:white-label-product:add', '', 3, 2, 1, 0, 0),
  (3284, 3090, 2, '修改白标产品', 3, 'PUT', '/core/white-label/products/:id', '', 'core:white-label-product:update', '', 4, 2, 1, 0, 0),
  (3285, 3090, 2, '修改白标产品状态', 3, 'POST', '/core/white-label/products/:id/status', '', 'core:white-label-product:status', '', 5, 2, 1, 0, 0),
  (3286, 3090, 2, '预检白标产品', 3, 'POST', '/core/white-label/products/:id/preflight', '', 'core:white-label-product:preflight', '', 6, 2, 1, 0, 0),
  (3287, 3090, 2, '删除白标产品', 3, 'DELETE', '/core/white-label/products/:id', '', 'core:white-label-product:delete', '', 7, 2, 1, 0, 0)
ON DUPLICATE KEY UPDATE
  name = VALUES(name), parent_id = VALUES(parent_id), app_scope = VALUES(app_scope), method = VALUES(method),
  path = VALUES(path), component = VALUES(component), perms = VALUES(perms), icon = VALUES(icon),
  sort = VALUES(sort), visible = VALUES(visible), enabled = VALUES(enabled);

INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.app_scope = r.app_scope
WHERE r.code IN ('owner', 'admin')
	AND m.id IN (17, 1081, 1082, 1083, 1084, 1085, 1086, 1087, 1101, 1102, 1103, 1104, 1105, 1106,
               18, 1091, 1092, 1093, 1094, 1095, 1096, 1097,
							 3080, 3271, 3272, 3273, 3274, 3275, 3276, 3277, 3291, 3292, 3293, 3294, 3295, 3296,
               3090, 3281, 3282, 3283, 3284, 3285, 3286, 3287);

INSERT IGNORE INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.app_scope = r.app_scope
WHERE r.code = 'viewer'
  AND m.id IN (17, 1081, 1082, 1085, 1104, 18, 1091, 1092,
               3080, 3271, 3272, 3275, 3294, 3090, 3281, 3282);

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260814_80_v3_white_label', 'V3高级白标模板、产品、动态包名与签名证书绑定')
ON DUPLICATE KEY UPDATE description = VALUES(description);
