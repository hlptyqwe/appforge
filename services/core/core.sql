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
-- 应用版本
-- =============================
DROP TABLE IF EXISTS t_app_version;
CREATE TABLE t_app_version (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '版本ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  version_code BIGINT NOT NULL COMMENT 'Android versionCode',
  version_name VARCHAR(64) NOT NULL COMMENT 'Android versionName',
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
  KEY idx_version_status (app_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='APK应用版本';


-- =============================
-- 签名配置
-- =============================
DROP TABLE IF EXISTS t_app_signing_config;
CREATE TABLE t_app_signing_config (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '签名配置ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  name VARCHAR(128) NOT NULL COMMENT '签名配置名称',
  keystore_object_key VARCHAR(500) NOT NULL COMMENT '私有对象存储中的keystore对象Key',
  key_alias VARCHAR(128) NOT NULL COMMENT '签名别名',
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
  KEY idx_signing_app_status (app_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='APK签名配置';


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
  source_apk_url VARCHAR(500) DEFAULT NULL COMMENT '构建时使用的原始APK地址快照',
  build_config JSON DEFAULT NULL COMMENT '构建时使用的构建配置快照',
  status VARCHAR(16) NOT NULL DEFAULT 'PENDING' COMMENT '构建状态',
  builder_id VARCHAR(128) DEFAULT NULL COMMENT 'Builder工作节点ID',
  builder_attempt INT NOT NULL DEFAULT 0 COMMENT 'Builder尝试次数',
  priority INT NOT NULL DEFAULT 0 COMMENT '任务优先级，值越大越优先',
  apk_url VARCHAR(500) DEFAULT NULL COMMENT '构建产物下载地址',
  apk_sha256 CHAR(64) DEFAULT NULL COMMENT '构建产物SHA-256',
  apk_size BIGINT NOT NULL DEFAULT 0 COMMENT '构建产物大小（字节）',
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
  KEY idx_build_builder_lease (builder_id, lease_until)
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
