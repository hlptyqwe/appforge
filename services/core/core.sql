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
-- 对象类型：1原始APK 2签名文件 3构建APK 4构建日志 5品牌Logo 6品牌启动图 7模板文件 8构建缓存 9离线任务包 10离线结果包
-- 状态：1上传中 2已就绪 3已绑定 4已删除 5失败
-- 存储模式：1控制面存储 2客户存储
-- =============================
DROP TABLE IF EXISTS t_storage_object;
CREATE TABLE t_storage_object (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '存储对象ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联应用ID，上传时允许为0',
  object_type TINYINT NOT NULL COMMENT '对象类型：1原始APK 2签名文件 3构建APK 4构建日志 5品牌Logo 6品牌启动图 7模板文件 8构建缓存 9离线任务包 10离线结果包',
  object_key VARCHAR(500) NOT NULL COMMENT '私有对象存储Key',
  original_name VARCHAR(255) NOT NULL COMMENT '用户上传时的原始文件名',
  content_type VARCHAR(128) NOT NULL DEFAULT 'application/octet-stream' COMMENT '对象内容类型',
  size_bytes BIGINT NOT NULL DEFAULT 0 COMMENT '对象大小，单位字节',
  sha256 CHAR(64) DEFAULT NULL COMMENT '对象SHA-256，小写十六进制',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1上传中 2已就绪 3已绑定 4已删除 5失败',
  storage_mode TINYINT NOT NULL DEFAULT 1 COMMENT '存储模式：1控制面存储 2客户存储',
  owner_agent_id BIGINT NOT NULL DEFAULT 0 COMMENT '客户存储所属Local Agent ID，控制面存储为0',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_storage_object_key (object_key) COMMENT '对象Key唯一索引',
  KEY idx_storage_tenant_type_status (tenant_id, object_type, status) COMMENT '租户对象类型状态查询索引',
  KEY idx_storage_tenant_app (tenant_id, app_id) COMMENT '租户应用对象查询索引',
  KEY idx_storage_mode_agent (tenant_id, storage_mode, owner_agent_id, status) COMMENT '客户存储对象归属和状态查询索引'
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
-- V4 Builder节点
-- 节点状态：1在线 2离线 3隔离
-- 排空状态：1接收任务 2排空中
-- =============================
DROP TABLE IF EXISTS t_builder_node;
CREATE TABLE t_builder_node (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Builder节点ID',
  node_code VARCHAR(64) NOT NULL COMMENT 'Builder节点唯一编码',
  pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT '节点所属构建池编码',
  endpoint VARCHAR(255) DEFAULT NULL COMMENT '节点管理端点或实例标识',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '节点状态：1在线 2离线 3隔离',
  drain_status TINYINT NOT NULL DEFAULT 1 COMMENT '排空状态：1接收任务 2排空中',
  max_concurrency INT NOT NULL DEFAULT 1 COMMENT '节点最大并发执行数',
  running_count INT NOT NULL DEFAULT 0 COMMENT '节点心跳上报的当前执行数',
  cpu_capacity INT NOT NULL DEFAULT 0 COMMENT 'CPU容量，单位毫核',
  memory_capacity BIGINT NOT NULL DEFAULT 0 COMMENT '内存容量，单位字节',
  disk_capacity BIGINT NOT NULL DEFAULT 0 COMMENT '构建磁盘总容量，单位字节',
  disk_free BIGINT NOT NULL DEFAULT 0 COMMENT '构建磁盘剩余容量，单位字节',
  toolchain_version VARCHAR(128) NOT NULL COMMENT '节点APK构建工具链版本',
  build_protocol_version INT NOT NULL DEFAULT 1 COMMENT '节点支持的构建协议版本',
  capability_json JSON DEFAULT NULL COMMENT '节点能力白名单JSON',
  consecutive_failures INT NOT NULL DEFAULT 0 COMMENT '节点连续失败次数',
  last_error_message VARCHAR(1000) DEFAULT NULL COMMENT '节点最近错误摘要',
  last_heartbeat_at DATETIME(3) NOT NULL COMMENT '节点最近心跳时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_builder_node_code (node_code) COMMENT '节点编码唯一',
  KEY idx_builder_node_pool_status (pool_code, status, drain_status) COMMENT '构建池可调度节点查询索引',
  KEY idx_builder_node_heartbeat (status, last_heartbeat_at) COMMENT '节点健康检查索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V4 Builder集群节点';


-- =============================
-- V4 构建并发与公平调度策略
-- 作用域：tenant_id=0且app_id=0为全局；tenant_id>0且app_id=0为租户；app_id>0为应用
-- 状态：1启用 2停用
-- =============================
DROP TABLE IF EXISTS t_build_concurrency_policy;
CREATE TABLE t_build_concurrency_policy (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '构建并发策略ID',
  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '租户ID，0表示全局作用域',
  app_id BIGINT NOT NULL DEFAULT 0 COMMENT '应用ID，0表示非应用作用域',
  pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT '策略适用构建池编码',
  max_concurrency INT NOT NULL COMMENT '作用域最大并发任务数',
  fair_weight INT NOT NULL DEFAULT 100 COMMENT '租户公平调度权重，基准值100',
  max_priority INT NOT NULL DEFAULT 100 COMMENT '作用域允许提交的最高普通优先级',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2停用',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_build_policy_scope (tenant_id, app_id, pool_code) COMMENT '并发策略作用域唯一',
  KEY idx_build_policy_pool_status (pool_code, status, tenant_id, app_id) COMMENT '构建池生效策略查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V4 构建并发与公平调度策略';


-- =============================
-- V4 公平队列状态
-- =============================
DROP TABLE IF EXISTS t_build_fair_queue;
CREATE TABLE t_build_fair_queue (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '公平队列状态ID',
  tenant_id BIGINT NOT NULL COMMENT '租户ID',
  pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT '构建池编码',
  virtual_finish BIGINT NOT NULL DEFAULT 0 COMMENT '加权公平队列虚拟完成时间',
  dispatch_count BIGINT NOT NULL DEFAULT 0 COMMENT '累计调度任务数',
  last_dispatched_at DATETIME(3) DEFAULT NULL COMMENT '最近调度时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_build_fair_tenant_pool (tenant_id, pool_code) COMMENT '租户构建池公平状态唯一',
  KEY idx_build_fair_pool_finish (pool_code, virtual_finish, last_dispatched_at) COMMENT '公平队列排序索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V4 租户加权公平队列状态';


-- =============================
-- V4 构建并发槽位租约
-- 状态：1生效 2已释放 3已过期 4已取消
-- =============================
DROP TABLE IF EXISTS t_build_slot_lease;
CREATE TABLE t_build_slot_lease (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '构建槽位租约ID',
  task_id BIGINT NOT NULL COMMENT '构建任务ID',
  tenant_id BIGINT NOT NULL COMMENT '租户ID',
  app_id BIGINT NOT NULL COMMENT '应用ID',
  node_code VARCHAR(64) NOT NULL COMMENT '占用槽位的Builder节点编码',
  pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT '构建池编码',
  builder_attempt INT NOT NULL COMMENT '任务领取代次',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1生效 2已释放 3已过期 4已取消',
  lease_until DATETIME(3) NOT NULL COMMENT '槽位租约到期时间',
  acquired_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '槽位获取时间',
  released_at DATETIME(3) DEFAULT NULL COMMENT '槽位释放时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_build_slot_task_attempt (task_id, builder_attempt) COMMENT '任务领取代次槽位唯一',
  KEY idx_build_slot_global (pool_code, status, lease_until) COMMENT '全局并发槽位查询索引',
  KEY idx_build_slot_tenant (tenant_id, pool_code, status, lease_until) COMMENT '租户并发槽位查询索引',
  KEY idx_build_slot_app (app_id, pool_code, status, lease_until) COMMENT '应用并发槽位查询索引',
  KEY idx_build_slot_node (node_code, status, lease_until) COMMENT '节点运行槽位查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V4 构建并发槽位租约';


-- =============================
-- V4 构建缓存
-- 缓存作用域：1租户隔离中间产物 2平台无敏感中间产物
-- 状态：1有效 2已失效 3已过期
-- =============================
DROP TABLE IF EXISTS t_build_cache_entry;
CREATE TABLE t_build_cache_entry (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '构建缓存条目ID',
  tenant_id BIGINT NOT NULL COMMENT '缓存所属租户ID，平台共享缓存为0',
  cache_scope TINYINT NOT NULL DEFAULT 1 COMMENT '缓存作用域：1租户隔离白标中间产物 2平台无敏感中间产物',
  cache_key CHAR(64) NOT NULL COMMENT '不可变构建输入SHA-256缓存键',
  toolchain_version VARCHAR(128) NOT NULL COMMENT '生成缓存的工具链版本',
  build_protocol_version INT NOT NULL DEFAULT 1 COMMENT '生成缓存的构建协议版本',
  input_digest CHAR(64) NOT NULL COMMENT '规范化不可变输入SHA-256',
  artifact_object_id BIGINT NOT NULL COMMENT '缓存产物存储对象ID',
  artifact_sha256 CHAR(64) NOT NULL COMMENT '缓存产物SHA-256',
  size_bytes BIGINT NOT NULL COMMENT '缓存产物大小，单位字节',
  hit_count BIGINT NOT NULL DEFAULT 0 COMMENT '累计缓存命中次数',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1有效 2已失效 3已过期',
  expires_at DATETIME(3) NOT NULL COMMENT '缓存过期时间',
  last_hit_at DATETIME(3) DEFAULT NULL COMMENT '最近命中时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_build_cache_scope_key (tenant_id, cache_scope, cache_key) COMMENT '缓存作用域键唯一',
  KEY idx_build_cache_lru (status, expires_at, last_hit_at, create_time) COMMENT '缓存TTL和LRU清理索引',
  KEY idx_build_cache_object (artifact_object_id, status) COMMENT '缓存对象引用查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V4 输入可寻址构建缓存';


-- =============================
-- V4 调度事件
-- 事件类型：1排队 2领取 3限流 4恢复 5排空 6取消 7重试 8缓存命中 9缓存失效 10完成 11失败
-- =============================
DROP TABLE IF EXISTS t_build_scheduler_event;
CREATE TABLE t_build_scheduler_event (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '构建调度事件ID',
  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联租户ID',
  app_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联应用ID',
  task_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联构建任务ID',
  node_code VARCHAR(64) DEFAULT NULL COMMENT '关联Builder节点编码',
  pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT '构建池编码',
  event_type TINYINT NOT NULL COMMENT '事件类型：1排队 2领取 3限流 4恢复 5排空 6取消 7重试 8缓存命中 9缓存失效 10完成 11失败',
  reason_code VARCHAR(64) DEFAULT NULL COMMENT '结构化调度原因编码',
  decision_json JSON DEFAULT NULL COMMENT '调度输入、限制和结果JSON',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '事件时间',
  PRIMARY KEY (id),
  KEY idx_scheduler_event_task (task_id, create_time) COMMENT '任务调度轨迹查询索引',
  KEY idx_scheduler_event_node (node_code, create_time) COMMENT '节点调度事件查询索引',
  KEY idx_scheduler_event_tenant_pool (tenant_id, pool_code, create_time) COMMENT '租户构建池事件查询索引',
  KEY idx_scheduler_event_type_time (event_type, create_time) COMMENT '事件类型时间查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V4 构建调度结构化事件';


-- =============================
-- V5 开放平台API凭证
-- 状态：1启用 2轮换过渡 3已吊销 4已过期
-- =============================
DROP TABLE IF EXISTS t_open_api_credential;
CREATE TABLE t_open_api_credential (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '开放API凭证ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  credential_name VARCHAR(128) NOT NULL COMMENT '凭证名称',
  key_id VARCHAR(32) NOT NULL COMMENT '公开Key标识',
  secret_hash CHAR(64) NOT NULL COMMENT '高熵Secret的SHA-256摘要，不保存明文',
  scopes JSON NOT NULL COMMENT '授权Scope字符串数组',
  app_ids JSON NOT NULL COMMENT '允许访问的应用ID数组，空数组表示租户内全部应用',
  ip_allowlist JSON NOT NULL COMMENT '允许访问的IP或CIDR数组，空数组表示不限制',
  rate_limit_per_minute INT NOT NULL DEFAULT 60 COMMENT '每分钟请求上限',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2轮换过渡 3已吊销 4已过期',
  expires_at DATETIME(3) DEFAULT NULL COMMENT '凭证到期时间，空表示长期有效',
  grace_expires_at DATETIME(3) DEFAULT NULL COMMENT '轮换过渡截止时间',
  rotated_from_id BIGINT NOT NULL DEFAULT 0 COMMENT '轮换来源凭证ID，0表示首次创建',
  last_used_at DATETIME(3) DEFAULT NULL COMMENT '最近成功使用时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_open_api_key_id (key_id) COMMENT 'Key标识全局唯一',
  KEY idx_open_api_credential_tenant_status (tenant_id, status, create_time) COMMENT '租户凭证状态查询索引',
  KEY idx_open_api_credential_rotation (rotated_from_id, status) COMMENT '凭证轮换链查询索引',
  KEY idx_open_api_credential_expiry (status, expires_at, grace_expires_at) COMMENT '凭证过期扫描索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 开放平台API凭证';

-- =============================
-- V5 Open API幂等结果
-- =============================
DROP TABLE IF EXISTS t_open_api_idempotency;
CREATE TABLE t_open_api_idempotency (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '幂等记录ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  credential_id BIGINT NOT NULL COMMENT '调用凭证ID',
  idempotency_key VARCHAR(128) NOT NULL COMMENT '客户端幂等键',
  request_method VARCHAR(16) NOT NULL COMMENT 'HTTP请求方法',
  request_path VARCHAR(255) NOT NULL COMMENT '规范化请求路径',
  request_hash CHAR(64) NOT NULL COMMENT '请求体SHA-256摘要',
  response_status INT NOT NULL DEFAULT 0 COMMENT 'HTTP响应状态码，0表示处理中',
  response_body JSON DEFAULT NULL COMMENT '可重放响应体',
  resource_type VARCHAR(64) DEFAULT NULL COMMENT '创建的资源类型',
  resource_id BIGINT NOT NULL DEFAULT 0 COMMENT '创建的资源ID',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1处理中 2已完成 3已失败',
  expires_at DATETIME(3) NOT NULL COMMENT '幂等记录过期时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_open_api_idempotency (tenant_id, credential_id, request_method, request_path, idempotency_key) COMMENT '凭证请求幂等键唯一',
  KEY idx_open_api_idempotency_expiry (status, expires_at) COMMENT '幂等记录清理索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 Open API幂等结果';

-- =============================
-- V5 Open API审计
-- =============================
DROP TABLE IF EXISTS t_open_api_audit;
CREATE TABLE t_open_api_audit (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '开放API审计ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  credential_id BIGINT NOT NULL COMMENT '调用凭证ID',
  key_id VARCHAR(32) NOT NULL COMMENT '调用Key标识快照',
  request_id VARCHAR(64) NOT NULL COMMENT '请求唯一标识',
  request_method VARCHAR(16) NOT NULL COMMENT 'HTTP请求方法',
  request_path VARCHAR(255) NOT NULL COMMENT '规范化请求路径',
  scope_used VARCHAR(64) DEFAULT NULL COMMENT '本次校验的Scope',
  resource_type VARCHAR(64) DEFAULT NULL COMMENT '访问资源类型',
  resource_id BIGINT NOT NULL DEFAULT 0 COMMENT '访问资源ID',
  client_ip VARCHAR(64) DEFAULT NULL COMMENT '调用方IP',
  response_status INT NOT NULL COMMENT 'HTTP响应状态码',
  duration_ms BIGINT NOT NULL DEFAULT 0 COMMENT '请求耗时毫秒',
  error_code VARCHAR(64) DEFAULT NULL COMMENT '结构化错误码',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '请求时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_open_api_audit_request (request_id) COMMENT '请求ID唯一',
  KEY idx_open_api_audit_tenant_time (tenant_id, create_time) COMMENT '租户审计时间查询索引',
  KEY idx_open_api_audit_credential_time (credential_id, create_time) COMMENT '凭证审计时间查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 Open API调用审计';

-- =============================
-- V5 Webhook端点
-- 状态：1启用 2暂停 3已吊销
-- =============================
DROP TABLE IF EXISTS t_webhook_endpoint;
CREATE TABLE t_webhook_endpoint (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Webhook端点ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  endpoint_name VARCHAR(128) NOT NULL COMMENT '端点名称',
  endpoint_url VARCHAR(1000) NOT NULL COMMENT 'HTTPS投递地址',
  event_types JSON NOT NULL COMMENT '订阅事件类型字符串数组',
  secret_ciphertext VARCHAR(1000) NOT NULL COMMENT '签名Secret的secretbox密文',
  secret_hint VARCHAR(16) NOT NULL COMMENT 'Secret末尾提示，不可用于验证',
  max_attempts INT NOT NULL DEFAULT 8 COMMENT '最大投递次数',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2暂停 3已吊销',
  last_success_at DATETIME(3) DEFAULT NULL COMMENT '最近投递成功时间',
  last_failure_at DATETIME(3) DEFAULT NULL COMMENT '最近投递失败时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_webhook_endpoint_tenant_status (tenant_id, status, create_time) COMMENT '租户端点状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 Webhook订阅端点';

-- =============================
-- V5 可靠业务事件Outbox
-- 状态：1待分发 2已分发 3已完成
-- =============================
DROP TABLE IF EXISTS t_outbox_event;
CREATE TABLE t_outbox_event (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Outbox事件ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  event_id CHAR(36) NOT NULL COMMENT '对外稳定事件UUID',
  event_type VARCHAR(64) NOT NULL COMMENT '事件类型：build.queued/build.started/build.succeeded/build.failed/build.canceled/artifact.expiring/quota.warning/quota.exceeded',
  aggregate_type VARCHAR(64) NOT NULL COMMENT '聚合根类型',
  aggregate_id BIGINT NOT NULL COMMENT '聚合根ID',
  schema_version INT NOT NULL DEFAULT 1 COMMENT '事件Schema版本',
  payload JSON NOT NULL COMMENT '事件业务数据JSON',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1待分发 2已分发 3已完成',
  occurred_at DATETIME(3) NOT NULL COMMENT '业务事件发生时间',
  dispatched_at DATETIME(3) DEFAULT NULL COMMENT '生成投递任务时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_outbox_event_id (event_id) COMMENT '对外事件ID唯一',
  KEY idx_outbox_status_time (status, occurred_at, id) COMMENT 'Outbox待分发扫描索引',
  KEY idx_outbox_tenant_type (tenant_id, event_type, occurred_at) COMMENT '租户事件查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 可靠业务事件Outbox';

-- =============================
-- V5 Webhook投递
-- 状态：1待投递 2投递中 3成功 4待重试 5死信
-- =============================
DROP TABLE IF EXISTS t_webhook_delivery;
CREATE TABLE t_webhook_delivery (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Webhook投递ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  endpoint_id BIGINT NOT NULL COMMENT 'Webhook端点ID',
  outbox_event_id BIGINT NOT NULL COMMENT 'Outbox事件ID',
  event_id CHAR(36) NOT NULL COMMENT '对外事件UUID快照',
  attempt INT NOT NULL DEFAULT 0 COMMENT '已执行尝试次数',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1待投递 2投递中 3成功 4待重试 5死信',
  response_status INT NOT NULL DEFAULT 0 COMMENT '最近HTTP响应状态码',
  response_body_excerpt VARCHAR(1000) DEFAULT NULL COMMENT '最近响应体摘要',
  error_message VARCHAR(1000) DEFAULT NULL COMMENT '最近错误摘要',
  next_retry_at DATETIME(3) NOT NULL COMMENT '下次允许投递时间',
  lease_until DATETIME(3) DEFAULT NULL COMMENT '投递Worker租约到期时间',
  delivered_at DATETIME(3) DEFAULT NULL COMMENT '投递成功时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_webhook_delivery_event_endpoint (outbox_event_id, endpoint_id) COMMENT '事件端点投递唯一',
  KEY idx_webhook_delivery_retry (status, next_retry_at, lease_until, id) COMMENT '投递重试领取索引',
  KEY idx_webhook_delivery_tenant_time (tenant_id, create_time) COMMENT '租户投递日志查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 Webhook投递记录';

-- =============================
-- V5 GitHub/GitLab集成
-- 平台：1GitHub 2GitLab；状态：1启用 2已断开 3错误
-- =============================
DROP TABLE IF EXISTS t_source_integration;
CREATE TABLE t_source_integration (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '代码平台集成ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  platform TINYINT NOT NULL COMMENT '代码平台：1GitHub 2GitLab',
  integration_name VARCHAR(128) NOT NULL COMMENT '集成名称',
  installation_ref VARCHAR(255) NOT NULL COMMENT 'OAuth或App安装标识',
  access_token_ciphertext VARCHAR(2000) NOT NULL COMMENT '最小权限访问令牌secretbox密文',
  refresh_token_ciphertext VARCHAR(2000) DEFAULT NULL COMMENT '刷新令牌secretbox密文',
  token_expires_at DATETIME(3) DEFAULT NULL COMMENT '访问令牌到期时间',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2已断开 3错误',
  last_sync_at DATETIME(3) DEFAULT NULL COMMENT '最近状态同步时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_source_integration_installation (tenant_id, platform, installation_ref) COMMENT '租户平台安装唯一',
  KEY idx_source_integration_tenant_status (tenant_id, status, platform) COMMENT '租户集成状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 GitHub和GitLab集成';

DROP TABLE IF EXISTS t_source_repository;
CREATE TABLE t_source_repository (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '授权仓库ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  integration_id BIGINT NOT NULL COMMENT '代码平台集成ID',
  external_repository_id VARCHAR(255) NOT NULL COMMENT '平台仓库ID',
  repository_full_name VARCHAR(500) NOT NULL COMMENT '仓库完整名称',
  default_branch VARCHAR(255) DEFAULT NULL COMMENT '默认分支',
  permission_level VARCHAR(32) NOT NULL DEFAULT 'read' COMMENT '授权级别：read',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1已授权 2已撤销',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_source_repository_external (integration_id, external_repository_id) COMMENT '集成仓库唯一',
  KEY idx_source_repository_tenant_status (tenant_id, status, repository_full_name) COMMENT '租户授权仓库查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 代码平台授权仓库';

DROP TABLE IF EXISTS t_source_artifact;
CREATE TABLE t_source_artifact (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '代码平台Artifact来源ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '关联应用ID',
  version_id BIGINT NOT NULL COMMENT '关联应用版本ID',
  integration_id BIGINT NOT NULL COMMENT '代码平台集成ID',
  repository_id BIGINT NOT NULL COMMENT '授权仓库ID',
  artifact_source TINYINT NOT NULL COMMENT 'Artifact来源：1Release 2CI任务',
  external_artifact_id VARCHAR(255) NOT NULL COMMENT '平台Artifact ID',
  commit_sha VARCHAR(64) NOT NULL COMMENT '来源Commit SHA',
  pipeline_ref VARCHAR(255) DEFAULT NULL COMMENT 'Pipeline或Workflow标识',
  job_ref VARCHAR(255) DEFAULT NULL COMMENT 'Job标识',
  artifact_sha256 CHAR(64) NOT NULL COMMENT '下载Artifact SHA-256',
  storage_object_id BIGINT NOT NULL COMMENT '导入后的原始APK对象ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_source_artifact_external (integration_id, external_artifact_id) COMMENT '平台Artifact唯一',
  KEY idx_source_artifact_version (tenant_id, version_id) COMMENT '版本来源查询索引',
  KEY idx_source_artifact_commit (repository_id, commit_sha) COMMENT '仓库Commit来源查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 代码平台APK Artifact来源';

DROP TABLE IF EXISTS t_source_build_trigger;
CREATE TABLE t_source_build_trigger (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '源码平台预定义构建触发策略ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  repository_id BIGINT NOT NULL COMMENT '已授权仓库ID',
  app_id BIGINT NOT NULL COMMENT '目标应用ID',
  trigger_name VARCHAR(128) NOT NULL COMMENT '触发策略名称',
  event_type TINYINT NOT NULL COMMENT '供应商事件类型：1发布Release 2成功CI流水线',
  ref_pattern VARCHAR(255) NOT NULL DEFAULT '*' COMMENT '允许触发的Tag或分支glob模式',
  artifact_selector VARCHAR(255) NOT NULL COMMENT 'APK附件名或CI Job名称精确选择器',
  channel_ids JSON NOT NULL COMMENT '目标渠道ID数组JSON',
  signing_config_id BIGINT NOT NULL COMMENT '签名配置ID',
  branding_profile_id BIGINT NOT NULL DEFAULT 0 COMMENT '品牌配置ID，0表示不使用',
  white_label_product_id BIGINT NOT NULL DEFAULT 0 COMMENT '白标产品ID，0表示不使用',
  priority TINYINT NOT NULL DEFAULT 2 COMMENT '构建优先级：0最低，数值越大优先级越高',
  pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT '目标构建池编码',
  version_name_prefix VARCHAR(32) NOT NULL DEFAULT '' COMMENT '自动版本名称前缀',
  webhook_token_hash CHAR(64) NOT NULL COMMENT '公开回调随机令牌SHA-256摘要',
  webhook_secret_ciphertext VARCHAR(2000) NOT NULL COMMENT '供应商Webhook签名Secret的secretbox密文',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '策略状态：1启用 2停用',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_source_build_trigger_name (tenant_id, trigger_name) COMMENT '租户触发策略名称唯一',
  UNIQUE KEY uk_source_build_trigger_token (webhook_token_hash) COMMENT '回调随机令牌摘要唯一',
  KEY idx_source_build_trigger_repository (tenant_id, repository_id, status) COMMENT '授权仓库启用策略查询索引',
  KEY idx_source_build_trigger_app (tenant_id, app_id, status) COMMENT '应用启用策略查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 源码平台预定义构建触发策略';

DROP TABLE IF EXISTS t_source_webhook_event;
CREATE TABLE t_source_webhook_event (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '源码平台入站Webhook事件ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  trigger_id BIGINT NOT NULL COMMENT '预定义构建触发策略ID',
  provider_event_id VARCHAR(255) NOT NULL COMMENT '供应商投递事件唯一ID',
  provider_event_type VARCHAR(64) NOT NULL COMMENT '供应商原始事件类型',
  source_ref VARCHAR(255) NOT NULL COMMENT '触发Tag或分支',
  commit_sha VARCHAR(64) NOT NULL COMMENT '来源Commit SHA',
  artifact_source TINYINT NOT NULL COMMENT 'Artifact来源：1Release附件 2CI任务Artifact',
  external_artifact_id VARCHAR(255) NOT NULL COMMENT '供应商Artifact或Job ID',
  release_ref VARCHAR(255) DEFAULT NULL COMMENT 'Release Tag，CI事件为空',
  pipeline_ref VARCHAR(255) DEFAULT NULL COMMENT 'Pipeline或Workflow标识',
  job_ref VARCHAR(255) DEFAULT NULL COMMENT 'Job标识',
  payload_sha256 CHAR(64) NOT NULL COMMENT '原始Webhook请求体SHA-256',
  version_code BIGINT NOT NULL COMMENT '事件入队时原子分配的Android versionCode',
  version_name VARCHAR(64) NOT NULL COMMENT '事件生成的Android versionName',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '处理状态：1待处理 2处理中 3成功 4忽略 5失败',
  attempt INT NOT NULL DEFAULT 0 COMMENT '处理尝试次数',
  claimed_by VARCHAR(128) DEFAULT NULL COMMENT '当前处理Worker实例标识',
  next_retry_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '下次允许处理时间',
  lease_until DATETIME(3) DEFAULT NULL COMMENT 'Worker处理租约到期时间',
  version_id BIGINT NOT NULL DEFAULT 0 COMMENT '导入成功后的版本ID',
  build_task_ids JSON DEFAULT NULL COMMENT '创建成功的构建任务ID数组JSON',
  error_message VARCHAR(1000) DEFAULT NULL COMMENT '最近失败错误摘要',
  completed_at DATETIME(3) DEFAULT NULL COMMENT '处理完成时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_source_webhook_event_delivery (trigger_id, provider_event_id) COMMENT '策略内供应商投递事件幂等',
  KEY idx_source_webhook_event_claim (status, next_retry_at, lease_until, id) COMMENT 'Worker可靠领取索引',
  KEY idx_source_webhook_event_tenant_time (tenant_id, create_time) COMMENT '租户事件审计查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 源码平台入站Webhook可靠事件';


-- =============================
-- V6 商业化套餐版本
-- 计费周期：1月付 2年付；状态：1启用 2退役
-- 构建计量规则：0不计费 1计费；额度值-1表示不限量
-- =============================
DROP TABLE IF EXISTS t_billing_plan;
CREATE TABLE t_billing_plan (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '套餐版本ID',
  plan_code VARCHAR(64) NOT NULL COMMENT '套餐稳定编码',
  plan_name VARCHAR(128) NOT NULL COMMENT '套餐展示名称',
  billing_cycle TINYINT NOT NULL COMMENT '计费周期：1月付 2年付',
  price_amount BIGINT NOT NULL DEFAULT 0 COMMENT '套餐价格，最小货币单位整数',
  currency CHAR(3) NOT NULL DEFAULT 'CNY' COMMENT 'ISO 4217大写币种',
  feature_json JSON NOT NULL COMMENT '非额度型功能开关JSON',
  builds_per_cycle BIGINT NOT NULL DEFAULT 0 COMMENT '每周期可计费构建次数，-1不限量',
  max_build_concurrency INT NOT NULL DEFAULT 1 COMMENT '租户最大构建并发，-1不限量',
  storage_bytes BIGINT NOT NULL DEFAULT 0 COMMENT '私有对象存储额度字节数，-1不限量',
  max_upload_bytes BIGINT NOT NULL DEFAULT 0 COMMENT '单文件上传上限字节数，-1不限量',
  team_seats INT NOT NULL DEFAULT 1 COMMENT '团队活跃席位数，-1不限量',
  api_rate_limit INT NOT NULL DEFAULT 60 COMMENT 'Open API每分钟请求上限，-1不限量',
  charge_failed_build TINYINT NOT NULL DEFAULT 0 COMMENT '失败构建计量规则：0不计费 1计费',
  charge_cache_hit TINYINT NOT NULL DEFAULT 1 COMMENT '缓存命中构建计量规则：0不计费 1计费',
  charge_retry_build TINYINT NOT NULL DEFAULT 1 COMMENT '重试构建计量规则：0不计费 1计费',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '套餐版本状态：1启用 2退役',
  version INT NOT NULL COMMENT '套餐不可变版本号，从1递增',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间，仅允许状态变化',
  PRIMARY KEY (id),
  UNIQUE KEY uk_billing_plan_code_version (plan_code, version) COMMENT '套餐编码版本唯一',
  KEY idx_billing_plan_status_cycle (status, billing_cycle, price_amount) COMMENT '可售套餐查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 不可变商业化套餐版本';

-- =============================
-- V6 租户订阅
-- 状态：1生效 2逾期 3宽限 4暂停 5已取消 6待支付
-- 来源：1Stripe 2人工合同 3平台赠送
-- =============================
DROP TABLE IF EXISTS t_tenant_subscription;
CREATE TABLE t_tenant_subscription (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '租户订阅ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  plan_id BIGINT NOT NULL COMMENT '当前套餐版本ID',
  plan_version INT NOT NULL COMMENT '订阅时固化的套餐版本号',
  status TINYINT NOT NULL COMMENT '订阅状态：1生效 2逾期 3宽限 4暂停 5已取消 6待支付',
  source TINYINT NOT NULL COMMENT '订阅来源：1Stripe 2人工合同 3平台赠送',
  external_customer_id VARCHAR(255) DEFAULT NULL COMMENT '支付提供商客户ID，人工订阅为空',
  external_subscription_id VARCHAR(255) DEFAULT NULL COMMENT '支付提供商订阅ID，人工订阅为空',
  current_period_start DATETIME(3) NOT NULL COMMENT '当前计费周期开始时间',
  current_period_end DATETIME(3) NOT NULL COMMENT '当前计费周期结束时间',
  cancel_at_period_end TINYINT NOT NULL DEFAULT 0 COMMENT '周期末取消：0否 1是',
  grace_until DATETIME(3) DEFAULT NULL COMMENT '宽限期截止时间',
  pending_plan_id BIGINT NOT NULL DEFAULT 0 COMMENT '周期末待切换套餐版本ID，0表示无',
  pending_plan_version INT NOT NULL DEFAULT 0 COMMENT '周期末待切换套餐版本号，0表示无',
  last_provider_event_at DATETIME(3) DEFAULT NULL COMMENT '最近已应用支付事件的供应商创建时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_subscription_tenant (tenant_id) COMMENT '每个租户一条当前订阅',
  UNIQUE KEY uk_tenant_subscription_external (source, external_subscription_id) COMMENT '供应商订阅ID唯一',
  KEY idx_tenant_subscription_status_period (status, current_period_end, grace_until) COMMENT '订阅到期与宽限扫描索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 租户当前订阅';

-- =============================
-- V6 租户当前权益快照
-- 来源：1套餐 2人工合同 3平台赠送；状态：1生效 2暂停
-- =============================
DROP TABLE IF EXISTS t_tenant_entitlement;
CREATE TABLE t_tenant_entitlement (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '租户权益快照ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  source_type TINYINT NOT NULL COMMENT '权益来源：1套餐 2人工合同 3平台赠送',
  source_id BIGINT NOT NULL COMMENT '来源订阅或人工授权ID',
  plan_id BIGINT NOT NULL COMMENT '权益基础套餐版本ID',
  plan_version INT NOT NULL COMMENT '权益基础套餐版本号',
  builds_per_cycle BIGINT NOT NULL COMMENT '每周期可计费构建次数，-1不限量',
  max_build_concurrency INT NOT NULL COMMENT '租户最大构建并发，-1不限量',
  storage_bytes BIGINT NOT NULL COMMENT '存储额度字节数，-1不限量',
  max_upload_bytes BIGINT NOT NULL COMMENT '单文件上传上限字节数，-1不限量',
  team_seats INT NOT NULL COMMENT '团队活跃席位数，-1不限量',
  api_rate_limit INT NOT NULL COMMENT 'Open API每分钟请求上限，-1不限量',
  charge_failed_build TINYINT NOT NULL COMMENT '失败构建计量规则：0不计费 1计费',
  charge_cache_hit TINYINT NOT NULL COMMENT '缓存命中构建计量规则：0不计费 1计费',
  charge_retry_build TINYINT NOT NULL COMMENT '重试构建计量规则：0不计费 1计费',
  override_json JSON DEFAULT NULL COMMENT '人工临时额度和功能覆盖JSON',
  valid_from DATETIME(3) NOT NULL COMMENT '权益生效时间',
  valid_until DATETIME(3) NOT NULL COMMENT '权益失效时间',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '权益状态：1生效 2暂停',
  revision BIGINT NOT NULL DEFAULT 1 COMMENT '权益快照修订号，每次变更递增',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_entitlement_tenant (tenant_id) COMMENT '每个租户一条当前权益快照',
  KEY idx_tenant_entitlement_status_validity (status, valid_until) COMMENT '权益状态与有效期查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 租户当前权益快照';

-- =============================
-- V6 不可变用量账本
-- 指标：build.started/build.succeeded/build.compute_seconds/storage.source_bytes/
-- storage.artifact_bytes/storage.log_bytes/api.requests/team.active_seats
-- =============================
DROP TABLE IF EXISTS t_usage_ledger;
CREATE TABLE t_usage_ledger (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '用量账本ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  metric VARCHAR(64) NOT NULL COMMENT '计量指标枚举字符串',
  quantity BIGINT NOT NULL COMMENT '本次用量增量，调整账可为负数',
  resource_type VARCHAR(64) NOT NULL COMMENT '来源资源类型',
  resource_id BIGINT NOT NULL DEFAULT 0 COMMENT '来源资源ID',
  idempotency_key VARCHAR(191) NOT NULL COMMENT '指标内幂等键',
  occurred_at DATETIME(3) NOT NULL COMMENT '业务发生时间',
  period_key CHAR(7) NOT NULL COMMENT '归属周期键YYYY-MM',
  metadata JSON DEFAULT NULL COMMENT '计量规则、缓存、重试或调整原因JSON',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_usage_ledger_idempotency (tenant_id, metric, idempotency_key) COMMENT '租户指标幂等键唯一',
  KEY idx_usage_ledger_period_metric (tenant_id, period_key, metric, occurred_at) COMMENT '租户周期指标汇总索引',
  KEY idx_usage_ledger_resource (resource_type, resource_id, metric) COMMENT '资源计量追溯索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 不可变用量账本';

-- =============================
-- V6 并发安全额度预占
-- 指标：build.count/storage.bytes/team.seats；状态：1预占 2确认 3释放 4过期
-- =============================
DROP TABLE IF EXISTS t_quota_reservation;
CREATE TABLE t_quota_reservation (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '额度预占ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  metric VARCHAR(64) NOT NULL COMMENT '预占额度指标枚举字符串',
  quantity BIGINT NOT NULL COMMENT '预占数量，必须大于0',
  resource_type VARCHAR(64) NOT NULL COMMENT '预占资源类型',
  resource_id BIGINT NOT NULL DEFAULT 0 COMMENT '预占资源ID，创建前可为0',
  idempotency_key VARCHAR(191) NOT NULL COMMENT '预占幂等键',
  period_key CHAR(7) NOT NULL COMMENT '归属周期键YYYY-MM',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '预占状态：1预占 2确认 3释放 4过期',
  expires_at DATETIME(3) NOT NULL COMMENT '未确认预占过期时间',
  confirmed_at DATETIME(3) DEFAULT NULL COMMENT '确认时间',
  released_at DATETIME(3) DEFAULT NULL COMMENT '释放或过期时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_quota_reservation_idempotency (tenant_id, metric, idempotency_key) COMMENT '租户指标预占幂等键唯一',
  KEY idx_quota_reservation_active (tenant_id, metric, period_key, status, expires_at) COMMENT '生效预占汇总索引',
  KEY idx_quota_reservation_resource (resource_type, resource_id, status) COMMENT '资源预占确认释放索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 并发安全额度预占';

-- =============================
-- V6 用量阈值通知
-- 阈值：70/90/100；状态：1待发送 2已发送 3发送失败
-- =============================
DROP TABLE IF EXISTS t_usage_threshold_notification;
CREATE TABLE t_usage_threshold_notification (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '阈值通知ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  metric VARCHAR(64) NOT NULL COMMENT '用量指标枚举字符串',
  period_key CHAR(7) NOT NULL COMMENT '归属周期键YYYY-MM',
  threshold_percent INT NOT NULL COMMENT '触发阈值百分比：70、90或100',
  usage_quantity BIGINT NOT NULL COMMENT '触发时当前用量',
  limit_quantity BIGINT NOT NULL COMMENT '触发时权益限额',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '通知状态：1待发送 2已发送 3发送失败',
  outbox_event_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联quota Webhook Outbox事件ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_usage_threshold_once (tenant_id, metric, period_key, threshold_percent) COMMENT '租户周期指标阈值仅通知一次',
  KEY idx_usage_threshold_status (status, create_time) COMMENT '通知发送状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 用量阈值幂等通知';

-- =============================
-- V6 不可变账单
-- 状态：1草稿 2待支付 3已支付 4支付失败 5已作废 6已退款
-- =============================
DROP TABLE IF EXISTS t_invoice;
CREATE TABLE t_invoice (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '账单ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  subscription_id BIGINT NOT NULL COMMENT '关联订阅ID',
  invoice_no VARCHAR(64) NOT NULL COMMENT '平台账单号',
  external_invoice_id VARCHAR(255) DEFAULT NULL COMMENT '支付提供商账单ID',
  status TINYINT NOT NULL COMMENT '账单状态：1草稿 2待支付 3已支付 4支付失败 5已作废 6已退款',
  currency CHAR(3) NOT NULL COMMENT 'ISO 4217大写币种',
  subtotal_amount BIGINT NOT NULL COMMENT '税前小计，最小货币单位整数',
  discount_amount BIGINT NOT NULL DEFAULT 0 COMMENT '折扣金额，最小货币单位整数',
  tax_amount BIGINT NOT NULL DEFAULT 0 COMMENT '税费，最小货币单位整数',
  total_amount BIGINT NOT NULL COMMENT '应付总额，最小货币单位整数',
  paid_amount BIGINT NOT NULL DEFAULT 0 COMMENT '已支付金额，最小货币单位整数',
  refunded_amount BIGINT NOT NULL DEFAULT 0 COMMENT '已退款金额，最小货币单位整数',
  period_start DATETIME(3) NOT NULL COMMENT '账单周期开始时间',
  period_end DATETIME(3) NOT NULL COMMENT '账单周期结束时间',
  due_at DATETIME(3) DEFAULT NULL COMMENT '最晚付款时间',
  paid_at DATETIME(3) DEFAULT NULL COMMENT '支付完成时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '状态更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_invoice_no (invoice_no) COMMENT '平台账单号唯一',
  UNIQUE KEY uk_invoice_external (external_invoice_id) COMMENT '供应商账单ID唯一',
  KEY idx_invoice_tenant_time (tenant_id, create_time) COMMENT '租户账单列表索引',
  KEY idx_invoice_status_due (status, due_at) COMMENT '待支付账单扫描索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 不可变账单';

-- =============================
-- V6 不可变账单项
-- 类型：1套餐 2用量 3折扣 4税费 5调整 6退款
-- =============================
DROP TABLE IF EXISTS t_invoice_item;
CREATE TABLE t_invoice_item (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '账单项ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  invoice_id BIGINT NOT NULL COMMENT '所属账单ID',
  line_key VARCHAR(191) NOT NULL COMMENT '账单内不可变行幂等键',
  item_type TINYINT NOT NULL COMMENT '账单项类型：1套餐 2用量 3折扣 4税费 5调整 6退款',
  description VARCHAR(500) NOT NULL COMMENT '账单项说明',
  metric VARCHAR(64) DEFAULT NULL COMMENT '用量指标，非用量项为空',
  quantity BIGINT NOT NULL DEFAULT 1 COMMENT '计费数量整数',
  unit_amount BIGINT NOT NULL COMMENT '单价，最小货币单位整数',
  amount BIGINT NOT NULL COMMENT '行金额，最小货币单位整数',
  metadata JSON DEFAULT NULL COMMENT '套餐版本或用量账本范围JSON',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_invoice_item_line (invoice_id, line_key) COMMENT '账单内行幂等键唯一',
  KEY idx_invoice_item_tenant_invoice (tenant_id, invoice_id, id) COMMENT '租户账单项查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 不可变账单项';

-- =============================
-- V6 支付交易
-- 类型：1扣款 2退款 3争议；状态：1处理中 2成功 3失败 4已撤销
-- =============================
DROP TABLE IF EXISTS t_payment_transaction;
CREATE TABLE t_payment_transaction (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '支付交易ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  invoice_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联账单ID，无法关联时为0',
  provider VARCHAR(32) NOT NULL COMMENT '支付提供商枚举字符串：stripe/manual',
  provider_transaction_id VARCHAR(255) NOT NULL COMMENT '支付提供商交易ID',
  transaction_type TINYINT NOT NULL COMMENT '交易类型：1扣款 2退款 3争议',
  status TINYINT NOT NULL COMMENT '交易状态：1处理中 2成功 3失败 4已撤销',
  currency CHAR(3) NOT NULL COMMENT 'ISO 4217大写币种',
  amount BIGINT NOT NULL COMMENT '交易金额，最小货币单位整数',
  failure_code VARCHAR(64) DEFAULT NULL COMMENT '失败结构化代码',
  failure_message VARCHAR(500) DEFAULT NULL COMMENT '失败脱敏摘要',
  occurred_at DATETIME(3) NOT NULL COMMENT '供应商业务发生时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '状态更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_payment_provider_transaction (provider, provider_transaction_id, transaction_type) COMMENT '供应商交易类型幂等唯一',
  KEY idx_payment_tenant_time (tenant_id, occurred_at) COMMENT '租户支付流水查询索引',
  KEY idx_payment_invoice_status (invoice_id, status) COMMENT '账单支付状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 支付交易流水';

-- =============================
-- V6 支付回调可靠事件
-- 状态：1待处理 2已应用 3已忽略 4失败
-- =============================
DROP TABLE IF EXISTS t_billing_webhook_event;
CREATE TABLE t_billing_webhook_event (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '支付回调事件ID',
  provider VARCHAR(32) NOT NULL COMMENT '支付提供商枚举字符串：stripe',
  provider_event_id VARCHAR(255) NOT NULL COMMENT '支付提供商事件ID',
  event_type VARCHAR(128) NOT NULL COMMENT '支付提供商事件类型',
  event_created_at DATETIME(3) NOT NULL COMMENT '支付提供商事件创建时间',
  payload_sha256 CHAR(64) NOT NULL COMMENT '原始请求体SHA-256',
  payload_ciphertext MEDIUMTEXT NOT NULL COMMENT '原始请求体secretbox密文',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '处理状态：1待处理 2已应用 3已忽略 4失败',
  attempt INT NOT NULL DEFAULT 0 COMMENT '处理尝试次数',
  tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '解析后的租户ID，未知时为0',
  error_message VARCHAR(1000) DEFAULT NULL COMMENT '处理失败脱敏摘要',
  processed_at DATETIME(3) DEFAULT NULL COMMENT '处理完成时间',
  retain_until DATETIME(3) NOT NULL COMMENT '密文载荷保留截止时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_billing_webhook_provider_event (provider, provider_event_id) COMMENT '支付提供商事件幂等唯一',
  KEY idx_billing_webhook_status (status, event_created_at, id) COMMENT '支付事件处理索引',
  KEY idx_billing_webhook_retention (retain_until, status) COMMENT '支付载荷保留清理索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 支付回调可靠事件';


-- =============================
-- 构建任务
-- 状态：PENDING、BUILDING、SIGNING、UPLOADING、SUCCESS、FAILED、CANCELLED
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
  pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT 'V4任务目标构建池编码',
  cache_key CHAR(64) DEFAULT NULL COMMENT 'V4不可变输入构建缓存键',
  source_webhook_event_id BIGINT DEFAULT NULL COMMENT 'V5源码Webhook事件ID，普通构建为空',
  cache_entry_id BIGINT NOT NULL DEFAULT 0 COMMENT 'V4命中的构建缓存条目ID',
  cache_hit TINYINT NOT NULL DEFAULT 0 COMMENT 'V4缓存命中标记：0未命中 1命中',
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
  cancel_requested_at DATETIME(3) DEFAULT NULL COMMENT '取消请求时间',
  cancelled_at DATETIME(3) DEFAULT NULL COMMENT '任务取消完成时间',
  cancel_reason VARCHAR(500) DEFAULT NULL COMMENT '任务取消原因',
  retry_of_task_id BIGINT NOT NULL DEFAULT 0 COMMENT '重试来源任务ID，0表示非重试任务',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_build_queue (status, priority, queued_at, id),
  KEY idx_build_tenant_app (tenant_id, app_id, create_time),
  KEY idx_build_channel (channel_id, create_time),
  KEY idx_build_builder_lease (builder_id, lease_until),
  KEY idx_build_pool_queue (pool_code, status, priority, queued_at, id) COMMENT 'V4构建池调度队列索引',
  KEY idx_build_cache_key (tenant_id, cache_key, status) COMMENT 'V4任务缓存键查询索引',
  UNIQUE KEY uk_build_source_webhook_channel (source_webhook_event_id, channel_id) COMMENT '源码Webhook事件每个渠道仅创建一个任务',
  KEY idx_build_retry_source (retry_of_task_id, create_time) COMMENT 'V4任务重试链查询索引',
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


-- =============================
-- V7 Local Builder Agent
-- Agent状态：1待注册 2在线 3离线 4已吊销 5需升级
-- Drain状态：1接单 2排空中 3已排空
-- Artifact模式：1控制面存储 2客户存储 3离线包
-- =============================
DROP TABLE IF EXISTS t_local_agent;
CREATE TABLE t_local_agent (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Local Agent ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  agent_code VARCHAR(64) NOT NULL COMMENT '租户内稳定Agent编码',
  agent_name VARCHAR(128) NOT NULL COMMENT 'Agent展示名称',
  pool_code VARCHAR(64) NOT NULL DEFAULT 'local' COMMENT '允许领取任务的构建池编码',
  status TINYINT NOT NULL DEFAULT 1 COMMENT 'Agent状态：1待注册 2在线 3离线 4已吊销 5需升级',
  drain_status TINYINT NOT NULL DEFAULT 1 COMMENT 'Drain状态：1接单 2排空中 3已排空',
  protocol_version INT NOT NULL DEFAULT 1 COMMENT 'Agent通信协议版本',
  agent_version VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'Agent语义化产品版本',
  artifact_mode TINYINT NOT NULL DEFAULT 1 COMMENT 'Artifact模式：1控制面存储 2客户存储 3离线包',
  customer_storage_ref VARCHAR(500) DEFAULT NULL COMMENT '客户存储Secret引用，不保存访问密钥',
  allowed_app_ids JSON NOT NULL COMMENT '允许构建的应用ID JSON数组',
  certificate_serial VARCHAR(128) DEFAULT NULL COMMENT '当前客户端证书序列号',
  last_nonce VARCHAR(128) DEFAULT NULL COMMENT '最近接受的请求Nonce，用于防重放',
  last_request_at DATETIME(3) DEFAULT NULL COMMENT '最近接受的请求时间，用于拒绝乱序重放',
  last_heartbeat_at DATETIME(3) DEFAULT NULL COMMENT '最近心跳时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_local_agent_tenant_code (tenant_id,agent_code) COMMENT '租户内Agent编码唯一',
  KEY idx_local_agent_status_heartbeat (status,last_heartbeat_at) COMMENT 'Agent状态心跳扫描索引',
  KEY idx_local_agent_pool_drain (tenant_id,pool_code,drain_status,status) COMMENT '租户构建池可调度Agent索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 客户网络Local Builder Agent';

DROP TABLE IF EXISTS t_local_agent_registration;
CREATE TABLE t_local_agent_registration (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '一次性注册记录ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  agent_id BIGINT NOT NULL COMMENT '关联Local Agent ID',
  token_hash CHAR(64) NOT NULL COMMENT '一次性注册码SHA-256摘要',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '注册码状态：1待使用 2已使用 3已过期 4已吊销',
  expires_at DATETIME(3) NOT NULL COMMENT '注册码过期时间',
  used_at DATETIME(3) DEFAULT NULL COMMENT '首次成功注册时间',
  used_ip VARCHAR(64) DEFAULT NULL COMMENT '首次成功注册来源IP',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_local_agent_registration_hash (token_hash) COMMENT '注册码摘要全局唯一',
  KEY idx_local_agent_registration_agent (tenant_id,agent_id,status,expires_at) COMMENT 'Agent有效注册码查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 Local Agent一次性注册凭证';

DROP TABLE IF EXISTS t_local_agent_certificate;
CREATE TABLE t_local_agent_certificate (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Agent客户端证书记录ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  agent_id BIGINT NOT NULL COMMENT '关联Local Agent ID',
  serial_number VARCHAR(128) NOT NULL COMMENT 'X.509证书序列号十六进制',
  fingerprint_sha256 CHAR(64) NOT NULL COMMENT '证书DER的SHA-256指纹',
  certificate_pem TEXT NOT NULL COMMENT '公开客户端证书PEM，不包含私钥',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '证书状态：1有效 2已轮换 3已吊销 4已过期',
  not_before DATETIME(3) NOT NULL COMMENT '证书生效时间',
  not_after DATETIME(3) NOT NULL COMMENT '证书失效时间',
  revoked_at DATETIME(3) DEFAULT NULL COMMENT '证书吊销时间',
  revoke_reason VARCHAR(500) DEFAULT NULL COMMENT '证书吊销原因',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_local_agent_certificate_serial (serial_number) COMMENT '证书序列号唯一',
  UNIQUE KEY uk_local_agent_certificate_fingerprint (fingerprint_sha256) COMMENT '证书指纹唯一',
  KEY idx_local_agent_certificate_agent (tenant_id,agent_id,status,not_after) COMMENT 'Agent有效证书查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 Local Agent公开客户端证书与吊销状态';

DROP TABLE IF EXISTS t_local_agent_capability;
CREATE TABLE t_local_agent_capability (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Agent能力记录ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  agent_id BIGINT NOT NULL COMMENT '关联Local Agent ID',
  capability_key VARCHAR(128) NOT NULL COMMENT '预定义能力键，不允许任意命令',
  capability_value VARCHAR(500) NOT NULL COMMENT '能力版本或约束值',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_local_agent_capability (agent_id,capability_key) COMMENT 'Agent能力键唯一',
  KEY idx_local_agent_capability_tenant (tenant_id,capability_key) COMMENT '租户能力匹配索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 Local Agent预定义能力';

DROP TABLE IF EXISTS t_hybrid_artifact_reference;
CREATE TABLE t_hybrid_artifact_reference (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '混合部署Artifact引用ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  agent_id BIGINT NOT NULL COMMENT '关联Local Agent ID',
  task_id BIGINT NOT NULL COMMENT '关联构建任务ID',
  builder_attempt INT NOT NULL COMMENT '构建任务fencing尝试次数',
  artifact_type TINYINT NOT NULL COMMENT 'Artifact类型：1源APK 2构建APK 3构建日志 4离线任务包',
  storage_mode TINYINT NOT NULL COMMENT '存储模式：1控制面存储 2客户存储 3离线包',
  object_reference VARCHAR(1000) NOT NULL COMMENT '对象引用或离线包ID，不包含访问凭证',
  sha256 CHAR(64) NOT NULL COMMENT 'Artifact内容SHA-256',
  size_bytes BIGINT NOT NULL COMMENT 'Artifact大小字节数',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '引用状态：1待校验 2已验证 3已失效',
  verified_at DATETIME(3) DEFAULT NULL COMMENT '大小、SHA、租户和attempt验证时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_hybrid_artifact_attempt_type (tenant_id,task_id,builder_attempt,artifact_type) COMMENT '任务attempt内Artifact类型唯一',
  KEY idx_hybrid_artifact_agent (tenant_id,agent_id,status,create_time) COMMENT 'Agent Artifact引用查询索引',
  KEY idx_hybrid_artifact_sha (sha256,size_bytes) COMMENT 'Artifact完整性追溯索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 Hybrid和离线Artifact授权引用';

-- APPFORGE_SCHEMA_113_BEGIN：历史目标112迁移镜像排除此Schema 113块
CREATE TABLE IF NOT EXISTS t_air_gapped_package (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'AIR_GAPPED离线包记录ID',
  tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  app_id BIGINT NOT NULL COMMENT '关联应用ID',
  package_code VARCHAR(128) NOT NULL COMMENT '离线包全局唯一编码',
  agent_id BIGINT NOT NULL COMMENT '目标Local Agent ID',
  task_id BIGINT NOT NULL COMMENT '关联构建任务ID',
  builder_attempt INT NOT NULL COMMENT '构建任务fencing尝试次数',
  agent_certificate_serial VARCHAR(128) NOT NULL COMMENT '导出时绑定的Agent客户端证书序列号',
  nonce_hash CHAR(64) NOT NULL COMMENT '一次性防重放Nonce的SHA-256摘要',
  export_object_id BIGINT NOT NULL DEFAULT 0 COMMENT '控制面离线任务ZIP对象ID',
  export_sha256 CHAR(64) DEFAULT NULL COMMENT '离线任务ZIP内容SHA-256',
  export_size_bytes BIGINT NOT NULL DEFAULT 0 COMMENT '离线任务ZIP大小字节数',
  result_object_id BIGINT NOT NULL DEFAULT 0 COMMENT 'Agent离线结果ZIP对象ID',
  result_sha256 CHAR(64) DEFAULT NULL COMMENT '离线结果ZIP内容SHA-256',
  result_size_bytes BIGINT NOT NULL DEFAULT 0 COMMENT '离线结果ZIP大小字节数',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '离线包状态：1准备中 2已导出 3已导入 4已过期 5已撤销',
  issued_at DATETIME(3) NOT NULL COMMENT '任务包签发时间',
  expires_at DATETIME(3) NOT NULL COMMENT '任务包过期时间',
  imported_at DATETIME(3) DEFAULT NULL COMMENT '结果包成功导入时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_air_gapped_package_code (package_code) COMMENT '离线包编码全局唯一',
  UNIQUE KEY uk_air_gapped_task_attempt (tenant_id,task_id,builder_attempt) COMMENT '任务attempt离线包唯一',
  KEY idx_air_gapped_agent_status (tenant_id,agent_id,status,expires_at) COMMENT 'Agent离线包状态查询索引',
  KEY idx_air_gapped_task_status (tenant_id,task_id,status) COMMENT '任务离线包状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 AIR_GAPPED离线任务与结果双向签名状态';
-- APPFORGE_SCHEMA_113_END
