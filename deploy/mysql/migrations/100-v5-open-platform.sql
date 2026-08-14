-- AppForge V5 开放平台、Webhook和代码平台集成。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

-- 新环境由core.sql初始化；已有环境使用以下与core.sql一致的显式DDL。
CREATE TABLE IF NOT EXISTS t_open_api_credential (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '开放API凭证ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID',
  credential_name VARCHAR(128) NOT NULL COMMENT '凭证名称', key_id VARCHAR(32) NOT NULL COMMENT '公开Key标识',
  secret_hash CHAR(64) NOT NULL COMMENT '高熵Secret的SHA-256摘要，不保存明文', scopes JSON NOT NULL COMMENT '授权Scope字符串数组',
  app_ids JSON NOT NULL COMMENT '允许访问的应用ID数组，空数组表示租户内全部应用', ip_allowlist JSON NOT NULL COMMENT '允许访问的IP或CIDR数组，空数组表示不限制',
  rate_limit_per_minute INT NOT NULL DEFAULT 60 COMMENT '每分钟请求上限', status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2轮换过渡 3已吊销 4已过期',
  expires_at DATETIME(3) DEFAULT NULL COMMENT '凭证到期时间', grace_expires_at DATETIME(3) DEFAULT NULL COMMENT '轮换过渡截止时间',
  rotated_from_id BIGINT NOT NULL DEFAULT 0 COMMENT '轮换来源凭证ID', last_used_at DATETIME(3) DEFAULT NULL COMMENT '最近成功使用时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id),
  UNIQUE KEY uk_open_api_key_id (key_id) COMMENT 'Key标识全局唯一', KEY idx_open_api_credential_tenant_status (tenant_id,status,create_time) COMMENT '租户凭证状态查询索引',
  KEY idx_open_api_credential_rotation (rotated_from_id,status) COMMENT '凭证轮换链查询索引', KEY idx_open_api_credential_expiry (status,expires_at,grace_expires_at) COMMENT '凭证过期扫描索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 开放平台API凭证';

CREATE TABLE IF NOT EXISTS t_open_api_idempotency (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '幂等记录ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', credential_id BIGINT NOT NULL COMMENT '调用凭证ID',
  idempotency_key VARCHAR(128) NOT NULL COMMENT '客户端幂等键', request_method VARCHAR(16) NOT NULL COMMENT 'HTTP请求方法', request_path VARCHAR(255) NOT NULL COMMENT '规范化请求路径',
  request_hash CHAR(64) NOT NULL COMMENT '请求体SHA-256摘要', response_status INT NOT NULL DEFAULT 0 COMMENT 'HTTP响应状态码', response_body JSON DEFAULT NULL COMMENT '可重放响应体',
  resource_type VARCHAR(64) DEFAULT NULL COMMENT '创建的资源类型', resource_id BIGINT NOT NULL DEFAULT 0 COMMENT '创建的资源ID', status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1处理中 2已完成 3已失败',
  expires_at DATETIME(3) NOT NULL COMMENT '幂等记录过期时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id),
  UNIQUE KEY uk_open_api_idempotency (tenant_id,credential_id,request_method,request_path,idempotency_key) COMMENT '凭证请求幂等键唯一',
  KEY idx_open_api_idempotency_expiry (status,expires_at) COMMENT '幂等记录清理索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 Open API幂等结果';

CREATE TABLE IF NOT EXISTS t_open_api_audit (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '开放API审计ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', credential_id BIGINT NOT NULL COMMENT '调用凭证ID', key_id VARCHAR(32) NOT NULL COMMENT '调用Key标识快照',
  request_id VARCHAR(64) NOT NULL COMMENT '请求唯一标识', request_method VARCHAR(16) NOT NULL COMMENT 'HTTP请求方法', request_path VARCHAR(255) NOT NULL COMMENT '规范化请求路径',
  scope_used VARCHAR(64) DEFAULT NULL COMMENT '本次校验的Scope', resource_type VARCHAR(64) DEFAULT NULL COMMENT '访问资源类型', resource_id BIGINT NOT NULL DEFAULT 0 COMMENT '访问资源ID',
  client_ip VARCHAR(64) DEFAULT NULL COMMENT '调用方IP', response_status INT NOT NULL COMMENT 'HTTP响应状态码', duration_ms BIGINT NOT NULL DEFAULT 0 COMMENT '请求耗时毫秒',
  error_code VARCHAR(64) DEFAULT NULL COMMENT '结构化错误码', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '请求时间', PRIMARY KEY (id),
  UNIQUE KEY uk_open_api_audit_request (request_id) COMMENT '请求ID唯一', KEY idx_open_api_audit_tenant_time (tenant_id,create_time) COMMENT '租户审计时间查询索引',
  KEY idx_open_api_audit_credential_time (credential_id,create_time) COMMENT '凭证审计时间查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 Open API调用审计';

CREATE TABLE IF NOT EXISTS t_webhook_endpoint (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Webhook端点ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', endpoint_name VARCHAR(128) NOT NULL COMMENT '端点名称', endpoint_url VARCHAR(1000) NOT NULL COMMENT 'HTTPS投递地址',
  event_types JSON NOT NULL COMMENT '订阅事件类型字符串数组', secret_ciphertext VARCHAR(1000) NOT NULL COMMENT '签名Secret的secretbox密文', secret_hint VARCHAR(16) NOT NULL COMMENT 'Secret末尾提示',
  max_attempts INT NOT NULL DEFAULT 8 COMMENT '最大投递次数', status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2暂停 3已吊销', last_success_at DATETIME(3) DEFAULT NULL COMMENT '最近投递成功时间',
  last_failure_at DATETIME(3) DEFAULT NULL COMMENT '最近投递失败时间', create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id),
  KEY idx_webhook_endpoint_tenant_status (tenant_id,status,create_time) COMMENT '租户端点状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 Webhook订阅端点';

CREATE TABLE IF NOT EXISTS t_outbox_event (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Outbox事件ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', event_id CHAR(36) NOT NULL COMMENT '对外稳定事件UUID', event_type VARCHAR(64) NOT NULL COMMENT '事件类型',
  aggregate_type VARCHAR(64) NOT NULL COMMENT '聚合根类型', aggregate_id BIGINT NOT NULL COMMENT '聚合根ID', schema_version INT NOT NULL DEFAULT 1 COMMENT '事件Schema版本', payload JSON NOT NULL COMMENT '事件业务数据JSON',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1待分发 2已分发 3已完成', occurred_at DATETIME(3) NOT NULL COMMENT '业务事件发生时间', dispatched_at DATETIME(3) DEFAULT NULL COMMENT '生成投递任务时间',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id),
  UNIQUE KEY uk_outbox_event_id (event_id) COMMENT '对外事件ID唯一', KEY idx_outbox_status_time (status,occurred_at,id) COMMENT 'Outbox待分发扫描索引', KEY idx_outbox_tenant_type (tenant_id,event_type,occurred_at) COMMENT '租户事件查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 可靠业务事件Outbox';

CREATE TABLE IF NOT EXISTS t_webhook_delivery (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Webhook投递ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', endpoint_id BIGINT NOT NULL COMMENT 'Webhook端点ID', outbox_event_id BIGINT NOT NULL COMMENT 'Outbox事件ID', event_id CHAR(36) NOT NULL COMMENT '对外事件UUID快照',
  attempt INT NOT NULL DEFAULT 0 COMMENT '已执行尝试次数', status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1待投递 2投递中 3成功 4待重试 5死信', response_status INT NOT NULL DEFAULT 0 COMMENT '最近HTTP响应状态码',
  response_body_excerpt VARCHAR(1000) DEFAULT NULL COMMENT '最近响应体摘要', error_message VARCHAR(1000) DEFAULT NULL COMMENT '最近错误摘要', next_retry_at DATETIME(3) NOT NULL COMMENT '下次允许投递时间',
  lease_until DATETIME(3) DEFAULT NULL COMMENT '投递Worker租约到期时间', delivered_at DATETIME(3) DEFAULT NULL COMMENT '投递成功时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id),
  UNIQUE KEY uk_webhook_delivery_event_endpoint (outbox_event_id,endpoint_id) COMMENT '事件端点投递唯一', KEY idx_webhook_delivery_retry (status,next_retry_at,lease_until,id) COMMENT '投递重试领取索引',
  KEY idx_webhook_delivery_tenant_time (tenant_id,create_time) COMMENT '租户投递日志查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 Webhook投递记录';

CREATE TABLE IF NOT EXISTS t_source_integration (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '代码平台集成ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', platform TINYINT NOT NULL COMMENT '代码平台：1GitHub 2GitLab', integration_name VARCHAR(128) NOT NULL COMMENT '集成名称',
  installation_ref VARCHAR(255) NOT NULL COMMENT 'OAuth或App安装标识', access_token_ciphertext VARCHAR(2000) NOT NULL COMMENT '访问令牌secretbox密文', refresh_token_ciphertext VARCHAR(2000) DEFAULT NULL COMMENT '刷新令牌secretbox密文',
  token_expires_at DATETIME(3) DEFAULT NULL COMMENT '访问令牌到期时间', status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1启用 2已断开 3错误', last_sync_at DATETIME(3) DEFAULT NULL COMMENT '最近状态同步时间',
  create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id),
  UNIQUE KEY uk_source_integration_installation (tenant_id,platform,installation_ref) COMMENT '租户平台安装唯一', KEY idx_source_integration_tenant_status (tenant_id,status,platform) COMMENT '租户集成状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 GitHub和GitLab集成';

CREATE TABLE IF NOT EXISTS t_source_repository (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '授权仓库ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', integration_id BIGINT NOT NULL COMMENT '代码平台集成ID', external_repository_id VARCHAR(255) NOT NULL COMMENT '平台仓库ID',
  repository_full_name VARCHAR(500) NOT NULL COMMENT '仓库完整名称', default_branch VARCHAR(255) DEFAULT NULL COMMENT '默认分支', permission_level VARCHAR(32) NOT NULL DEFAULT 'read' COMMENT '授权级别：read',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1已授权 2已撤销', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id),
  UNIQUE KEY uk_source_repository_external (integration_id,external_repository_id) COMMENT '集成仓库唯一', KEY idx_source_repository_tenant_status (tenant_id,status,repository_full_name) COMMENT '租户授权仓库查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 代码平台授权仓库';

CREATE TABLE IF NOT EXISTS t_source_artifact (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '代码平台Artifact来源ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', app_id BIGINT NOT NULL COMMENT '关联应用ID', version_id BIGINT NOT NULL COMMENT '关联应用版本ID',
  integration_id BIGINT NOT NULL COMMENT '代码平台集成ID', repository_id BIGINT NOT NULL COMMENT '授权仓库ID', artifact_source TINYINT NOT NULL COMMENT 'Artifact来源：1Release 2CI任务', external_artifact_id VARCHAR(255) NOT NULL COMMENT '平台Artifact ID',
  commit_sha VARCHAR(64) NOT NULL COMMENT '来源Commit SHA', pipeline_ref VARCHAR(255) DEFAULT NULL COMMENT 'Pipeline或Workflow标识', job_ref VARCHAR(255) DEFAULT NULL COMMENT 'Job标识', artifact_sha256 CHAR(64) NOT NULL COMMENT '下载Artifact SHA-256',
  storage_object_id BIGINT NOT NULL COMMENT '导入后的原始APK对象ID', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', PRIMARY KEY (id),
  UNIQUE KEY uk_source_artifact_external (integration_id,external_artifact_id) COMMENT '平台Artifact唯一', KEY idx_source_artifact_version (tenant_id,version_id) COMMENT '版本来源查询索引',
  KEY idx_source_artifact_commit (repository_id,commit_sha) COMMENT '仓库Commit来源查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V5 代码平台APK Artifact来源';

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260814_100_v5_open_platform', 'V5开放平台、Webhook和代码平台集成数据模型')
ON DUPLICATE KEY UPDATE description=VALUES(description);
