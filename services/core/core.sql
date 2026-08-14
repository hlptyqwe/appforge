-- ============================================================================
-- APK 渠道动态打包平台 Core 业务表
-- 说明：
-- 1. 以下表均保留 tenant_id，SaaS 和私有部署使用同一套数据结构。
-- 2. system.sql 只负责租户和RBAC，Core 负责这些业务表的关联生命周期。
-- 3. 业务表不建立跨服务外键，由 Core 负责租户隔离和关联数据生命周期管理。
-- 4. 所有敏感签名信息只保存加密值或外部密钥引用，不保存明文密码。
-- ============================================================================

-- =============================
-- 应用
-- =============================
DROP TABLE IF EXISTS t_app_application;
CREATE TABLE t_app_application (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '应用ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_code VARCHAR(64) NOT NULL COMMENT '应用编码',
  app_name VARCHAR(128) NOT NULL COMMENT '应用名称',
  package_name VARCHAR(255) NOT NULL COMMENT 'Android applicationId/packageName',
  description VARCHAR(500) DEFAULT NULL COMMENT '应用描述',
  icon_url VARCHAR(500) DEFAULT NULL COMMENT '应用图标地址',
  api_host VARCHAR(500) DEFAULT NULL COMMENT '应用API地址',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2停用',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_app_tenant_code (tenant_id, app_code),
  UNIQUE KEY uk_app_tenant_package (tenant_id, package_name),
  KEY idx_app_tenant_status (tenant_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='APK分发应用';


-- =============================
-- 对象存储元数据
-- 对象类型：1原始APK 2签名文件 3构建APK 4构建日志 5品牌Logo 6品牌启动图
-- 状态：1上传中 2已就绪 3已绑定 4已删除 5失败
-- =============================
DROP TABLE IF EXISTS t_storage_object;
CREATE TABLE t_storage_object (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '存储对象ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联应用ID，上传时允许为0',
  object_type TINYINT NOT NULL COMMENT '对象类型：1原始APK 2签名文件 3构建APK 4构建日志 5品牌Logo 6品牌启动图',
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


-- =============================
-- 应用版本
-- =============================
DROP TABLE IF EXISTS t_app_version;
CREATE TABLE t_app_version (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '版本ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  version_code BIGINT NOT NULL COMMENT 'Android versionCode',
  version_name VARCHAR(64) NOT NULL COMMENT 'Android versionName',
  source_apk_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '原始APK存储对象ID',
  source_apk_url VARCHAR(500) DEFAULT NULL COMMENT '原始APK地址',
  source_apk_sha256 CHAR(64) DEFAULT NULL COMMENT '原始APK SHA-256',
  release_notes TEXT COMMENT '版本说明',
  build_config JSON DEFAULT NULL COMMENT '构建配置',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1草稿 2已发布 3已停用',
  published_at DATETIME DEFAULT NULL COMMENT '发布时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_version_app_code (app_id, version_code),
  KEY idx_version_tenant_app (tenant_id, app_id),
  KEY idx_version_status (app_id, status),
  KEY idx_version_source_object (source_apk_object_id) COMMENT '原始APK对象查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='APK应用版本';


-- =============================
-- 应用品牌配置
-- 重写模式：1资源重建 2运行时契约
-- 状态：1草稿 2启用 3停用
-- =============================
DROP TABLE IF EXISTS t_app_branding_profile;
CREATE TABLE t_app_branding_profile (
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


-- =============================
-- 品牌兼容性预检
-- 状态：1待处理 2兼容 3不兼容 4执行失败
-- =============================
DROP TABLE IF EXISTS t_branding_preflight;
CREATE TABLE t_branding_preflight (
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


-- =============================
-- 签名配置
-- =============================
DROP TABLE IF EXISTS t_app_signing_config;
CREATE TABLE t_app_signing_config (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '签名配置ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  name VARCHAR(128) NOT NULL COMMENT '签名配置名称',
  keystore_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '签名文件存储对象ID',
  keystore_object_key VARCHAR(500) NOT NULL COMMENT '私有对象存储中的keystore对象Key',
  key_alias VARCHAR(128) NOT NULL COMMENT '签名别名',
  certificate_sha256 CHAR(64) DEFAULT NULL COMMENT '签名证书SHA-256指纹，小写十六进制',
  keystore_password_ciphertext VARCHAR(1024) DEFAULT NULL COMMENT 'keystore密码密文',
  key_password_ciphertext VARCHAR(1024) DEFAULT NULL COMMENT 'key密码密文',
  secret_ref VARCHAR(255) DEFAULT NULL COMMENT 'KMS/Secrets Manager/Vault引用',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2停用',
  last_verified_at DATETIME DEFAULT NULL COMMENT '最近验证时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_signing_tenant_app_name (tenant_id, app_id, name),
  KEY idx_signing_app_status (app_id, status),
  KEY idx_signing_keystore_object (keystore_object_id) COMMENT '签名文件对象查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='APK签名配置';


-- =============================
-- V3 白标模板
-- 状态：1草稿 2已发布 3停用
-- =============================
DROP TABLE IF EXISTS t_white_label_template;
CREATE TABLE t_white_label_template (
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


-- =============================
-- V3 白标模板不可变修订
-- 状态：1草稿 2已发布 3已取代
-- =============================
DROP TABLE IF EXISTS t_white_label_template_revision;
CREATE TABLE t_white_label_template_revision (
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


-- =============================
-- V3 白标产品
-- 状态：1草稿 2启用 3停用
-- =============================
DROP TABLE IF EXISTS t_white_label_product;
CREATE TABLE t_white_label_product (
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


-- =============================
-- V3 包名签名证书历史绑定
-- 状态：1生效 2变更待审批 3已废弃
-- =============================
DROP TABLE IF EXISTS t_package_certificate_binding;
CREATE TABLE t_package_certificate_binding (
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


-- =============================
-- 推广渠道
-- =============================
DROP TABLE IF EXISTS t_promotion_channel;
CREATE TABLE t_promotion_channel (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '渠道ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  channel_code VARCHAR(32) NOT NULL COMMENT '唯一渠道编码，写入APK的CHANNEL_CODE',
  channel_name VARCHAR(100) NOT NULL COMMENT '渠道名称',
  landing_url VARCHAR(500) DEFAULT NULL COMMENT '渠道推广页地址',
  download_url VARCHAR(500) DEFAULT NULL COMMENT '渠道APK下载地址',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2停用',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_channel_code (channel_code),
  UNIQUE KEY uk_channel_tenant_app_name (tenant_id, app_id, channel_name),
  KEY idx_channel_tenant_app (tenant_id, app_id),
  KEY idx_channel_app_status (app_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='APK推广渠道';


-- =============================
-- 构建任务
-- 状态：PENDING、BUILDING、SIGNING、UPLOADING、SUCCESS、FAILED
-- =============================
DROP TABLE IF EXISTS t_build_task;
CREATE TABLE t_build_task (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '构建任务ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  version_id BIGINT NOT NULL COMMENT '应用版本ID',
  channel_id BIGINT NOT NULL COMMENT '渠道ID',
  signing_config_id BIGINT NOT NULL COMMENT '签名配置ID',
  channel_code VARCHAR(32) NOT NULL COMMENT '构建时使用的渠道编码快照',
  version_code BIGINT NOT NULL COMMENT '构建时使用的versionCode快照',
  version_name VARCHAR(64) NOT NULL COMMENT '构建时使用的versionName快照',
  source_apk_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '构建时使用的原始APK对象ID快照',
  source_apk_url VARCHAR(500) DEFAULT NULL COMMENT '构建时使用的原始APK地址快照',
  build_config JSON DEFAULT NULL COMMENT '构建时使用的构建配置快照',
  branding_profile_id BIGINT NOT NULL DEFAULT 0 COMMENT '构建时使用的品牌配置ID，0表示不启用白标',
  branding_revision INT NOT NULL DEFAULT 0 COMMENT '构建时使用的品牌配置修订号',
  branding_snapshot JSON DEFAULT NULL COMMENT '构建时固化的品牌配置快照JSON',
  white_label_product_id BIGINT NOT NULL DEFAULT 0 COMMENT 'V3白标产品ID，0表示非高级白标构建',
  template_revision INT NOT NULL DEFAULT 0 COMMENT 'V3构建使用的模板修订号',
  template_snapshot JSON DEFAULT NULL COMMENT 'V3模板和产品不可变构建快照JSON',
  status VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT '构建状态',
  builder_id VARCHAR(128) DEFAULT NULL COMMENT 'Builder工作节点ID',
  builder_attempt INT NOT NULL DEFAULT 0 COMMENT 'Builder尝试次数',
  priority INT NOT NULL DEFAULT 0 COMMENT '任务优先级，值越大越优先',
  apk_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '构建产物APK存储对象ID',
  apk_url VARCHAR(500) DEFAULT NULL COMMENT '构建产物下载地址',
  apk_sha256 CHAR(64) DEFAULT NULL COMMENT '构建产物SHA-256',
  apk_size BIGINT NOT NULL DEFAULT 0 COMMENT '构建产物大小（字节）',
  log_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '构建日志存储对象ID',
  log_url VARCHAR(500) DEFAULT NULL COMMENT '构建日志地址',
  error_message VARCHAR(2000) DEFAULT NULL COMMENT '失败原因',
  queued_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '进入队列时间',
  start_time DATETIME DEFAULT NULL COMMENT '开始构建时间',
  finish_time DATETIME DEFAULT NULL COMMENT '完成时间',
  lease_until DATETIME DEFAULT NULL COMMENT 'Builder租约到期时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_build_queue (status, priority, queued_at, id),
  KEY idx_build_tenant_app (tenant_id, app_id, create_time),
  KEY idx_build_channel (channel_id, create_time),
  KEY idx_build_builder_lease (builder_id, lease_until),
  KEY idx_build_source_object (source_apk_object_id) COMMENT '构建源APK对象查询索引',
  KEY idx_build_branding_profile (branding_profile_id, branding_revision) COMMENT '品牌配置构建任务查询索引',
  KEY idx_build_white_label_product (white_label_product_id, template_revision) COMMENT '白标产品构建任务查询索引',
  KEY idx_build_apk_object (apk_object_id) COMMENT '构建产物对象查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='APK动态构建任务';


-- =============================
-- 渠道安装归因
-- 首次绑定后不允许客户端覆盖channel_code
-- =============================
DROP TABLE IF EXISTS t_channel_install;
CREATE TABLE t_channel_install (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '安装记录ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  channel_id BIGINT NOT NULL COMMENT '渠道ID',
  channel_code VARCHAR(32) NOT NULL COMMENT '渠道编码',
  install_id VARCHAR(128) NOT NULL COMMENT '客户端安装唯一ID',
  app_version VARCHAR(64) DEFAULT NULL COMMENT '客户端版本',
  device_model VARCHAR(128) DEFAULT NULL COMMENT '设备型号',
  ip VARCHAR(64) DEFAULT NULL COMMENT '首次启动IP',
  first_open_time DATETIME NOT NULL COMMENT '首次启动时间',
  register_user_id BIGINT DEFAULT NULL COMMENT '归因注册用户ID',
  register_time DATETIME DEFAULT NULL COMMENT '注册时间',
  first_pay_time DATETIME DEFAULT NULL COMMENT '首充时间',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_install_id (install_id),
  KEY idx_install_tenant_app (tenant_id, app_id, first_open_time),
  KEY idx_install_channel (channel_id, first_open_time),
  KEY idx_install_channel_code (channel_code),
  KEY idx_install_register_user (register_user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='渠道安装归因记录';


-- =============================
-- 渠道转化事件
-- event_type：1点击 2下载 3首次启动 4注册 5首充 6付费
-- event_key用于接口重试幂等，避免重复统计
-- =============================
DROP TABLE IF EXISTS t_channel_event;
CREATE TABLE t_channel_event (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '事件ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  channel_id BIGINT NOT NULL COMMENT '渠道ID',
  channel_code VARCHAR(32) NOT NULL COMMENT '渠道编码',
  event_type TINYINT NOT NULL COMMENT '事件类型：1点击 2下载 3首次启动 4注册 5首充 6付费',
  event_key VARCHAR(128) NOT NULL COMMENT '事件幂等键',
  install_id VARCHAR(128) DEFAULT NULL COMMENT '安装ID',
  user_id BIGINT DEFAULT NULL COMMENT '关联用户ID',
  app_version VARCHAR(64) DEFAULT NULL COMMENT '客户端版本',
  ip VARCHAR(64) DEFAULT NULL COMMENT '事件IP',
  device_model VARCHAR(128) DEFAULT NULL COMMENT '设备型号',
  event_time DATETIME NOT NULL COMMENT '事件发生时间',
  metadata JSON DEFAULT NULL COMMENT '事件扩展信息',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '入库时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_channel_event_key (tenant_id, event_type, event_key),
  KEY idx_event_channel_time (channel_id, event_type, event_time),
  KEY idx_event_app_time (tenant_id, app_id, event_type, event_time),
  KEY idx_event_install (install_id),
  KEY idx_event_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='渠道点击下载及转化事件';
