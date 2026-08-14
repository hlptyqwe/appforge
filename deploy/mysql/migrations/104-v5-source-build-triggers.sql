-- AppForge V5 源码平台预定义构建触发策略与可靠入站事件。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

CREATE TABLE IF NOT EXISTS t_source_build_trigger (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '源码平台预定义构建触发策略ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', repository_id BIGINT NOT NULL COMMENT '已授权仓库ID', app_id BIGINT NOT NULL COMMENT '目标应用ID',
  trigger_name VARCHAR(128) NOT NULL COMMENT '触发策略名称', event_type TINYINT NOT NULL COMMENT '供应商事件类型：1发布Release 2成功CI流水线', ref_pattern VARCHAR(255) NOT NULL DEFAULT '*' COMMENT '允许触发的Tag或分支glob模式',
  artifact_selector VARCHAR(255) NOT NULL COMMENT 'APK附件名或CI Job名称精确选择器', channel_ids JSON NOT NULL COMMENT '目标渠道ID数组JSON', signing_config_id BIGINT NOT NULL COMMENT '签名配置ID',
  branding_profile_id BIGINT NOT NULL DEFAULT 0 COMMENT '品牌配置ID，0表示不使用', white_label_product_id BIGINT NOT NULL DEFAULT 0 COMMENT '白标产品ID，0表示不使用', priority TINYINT NOT NULL DEFAULT 2 COMMENT '构建优先级：0最低，数值越大优先级越高',
  pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT '目标构建池编码', version_name_prefix VARCHAR(32) NOT NULL DEFAULT '' COMMENT '自动版本名称前缀', webhook_token_hash CHAR(64) NOT NULL COMMENT '公开回调随机令牌SHA-256摘要',
  webhook_secret_ciphertext VARCHAR(2000) NOT NULL COMMENT '供应商Webhook签名Secret的secretbox密文', status TINYINT NOT NULL DEFAULT 1 COMMENT '策略状态：1启用 2停用', create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id),
  UNIQUE KEY uk_source_build_trigger_name (tenant_id,trigger_name) COMMENT '租户触发策略名称唯一', UNIQUE KEY uk_source_build_trigger_token (webhook_token_hash) COMMENT '回调随机令牌摘要唯一',
  KEY idx_source_build_trigger_repository (tenant_id,repository_id,status) COMMENT '授权仓库启用策略查询索引', KEY idx_source_build_trigger_app (tenant_id,app_id,status) COMMENT '应用启用策略查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 源码平台预定义构建触发策略';

CREATE TABLE IF NOT EXISTS t_source_webhook_event (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '源码平台入站Webhook事件ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', trigger_id BIGINT NOT NULL COMMENT '预定义构建触发策略ID', provider_event_id VARCHAR(255) NOT NULL COMMENT '供应商投递事件唯一ID',
  provider_event_type VARCHAR(64) NOT NULL COMMENT '供应商原始事件类型', source_ref VARCHAR(255) NOT NULL COMMENT '触发Tag或分支', commit_sha VARCHAR(64) NOT NULL COMMENT '来源Commit SHA', artifact_source TINYINT NOT NULL COMMENT 'Artifact来源：1Release附件 2CI任务Artifact',
  external_artifact_id VARCHAR(255) NOT NULL COMMENT '供应商Artifact或Job ID', release_ref VARCHAR(255) DEFAULT NULL COMMENT 'Release Tag，CI事件为空', pipeline_ref VARCHAR(255) DEFAULT NULL COMMENT 'Pipeline或Workflow标识', job_ref VARCHAR(255) DEFAULT NULL COMMENT 'Job标识',
  payload_sha256 CHAR(64) NOT NULL COMMENT '原始Webhook请求体SHA-256', version_code BIGINT NOT NULL COMMENT '事件入队时原子分配的Android versionCode', version_name VARCHAR(64) NOT NULL COMMENT '事件生成的Android versionName',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '处理状态：1待处理 2处理中 3成功 4忽略 5失败', attempt INT NOT NULL DEFAULT 0 COMMENT '处理尝试次数', claimed_by VARCHAR(128) DEFAULT NULL COMMENT '当前处理Worker实例标识', next_retry_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '下次允许处理时间',
  lease_until DATETIME(3) DEFAULT NULL COMMENT 'Worker处理租约到期时间', version_id BIGINT NOT NULL DEFAULT 0 COMMENT '导入成功后的版本ID', build_task_ids JSON DEFAULT NULL COMMENT '创建成功的构建任务ID数组JSON',
  error_message VARCHAR(1000) DEFAULT NULL COMMENT '最近失败错误摘要', completed_at DATETIME(3) DEFAULT NULL COMMENT '处理完成时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id), UNIQUE KEY uk_source_webhook_event_delivery (trigger_id,provider_event_id) COMMENT '策略内供应商投递事件幂等',
  KEY idx_source_webhook_event_claim (status,next_retry_at,lease_until,id) COMMENT 'Worker可靠领取索引', KEY idx_source_webhook_event_tenant_time (tenant_id,create_time) COMMENT '租户事件审计查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 源码平台入站Webhook可靠事件';

SET @source_webhook_column_exists = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 't_build_task' AND column_name = 'source_webhook_event_id'
);
SET @source_webhook_column_sql = IF(
  @source_webhook_column_exists = 0,
  'ALTER TABLE t_build_task ADD COLUMN source_webhook_event_id BIGINT NULL COMMENT ''V5源码Webhook事件ID，普通构建为空'' AFTER cache_key',
  'SELECT 1'
);
PREPARE source_webhook_column_stmt FROM @source_webhook_column_sql;
EXECUTE source_webhook_column_stmt;
DEALLOCATE PREPARE source_webhook_column_stmt;

SET @source_webhook_index_exists = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 't_build_task' AND index_name = 'uk_build_source_webhook_channel'
);
SET @source_webhook_index_sql = IF(
  @source_webhook_index_exists = 0,
  'ALTER TABLE t_build_task ADD UNIQUE KEY uk_build_source_webhook_channel (source_webhook_event_id, channel_id) COMMENT ''源码Webhook事件每个渠道仅创建一个任务''',
  'SELECT 1'
);
PREPARE source_webhook_index_stmt FROM @source_webhook_index_sql;
EXECUTE source_webhook_index_stmt;
DEALLOCATE PREPARE source_webhook_index_stmt;

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_104_v5_source_build_triggers','V5源码平台预定义构建触发策略与可靠入站事件')
ON DUPLICATE KEY UPDATE description=VALUES(description);
