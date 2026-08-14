-- AppForge V7 企业交付与 Local Builder Agent。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

CREATE TABLE IF NOT EXISTS t_local_agent (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Local Agent ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', agent_code VARCHAR(64) NOT NULL COMMENT '租户内稳定Agent编码', agent_name VARCHAR(128) NOT NULL COMMENT 'Agent展示名称', pool_code VARCHAR(64) NOT NULL DEFAULT 'local' COMMENT '允许领取任务的构建池编码', status TINYINT NOT NULL DEFAULT 1 COMMENT 'Agent状态：1待注册 2在线 3离线 4已吊销 5需升级', drain_status TINYINT NOT NULL DEFAULT 1 COMMENT 'Drain状态：1接单 2排空中 3已排空', protocol_version INT NOT NULL DEFAULT 1 COMMENT 'Agent通信协议版本', agent_version VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'Agent语义化产品版本', artifact_mode TINYINT NOT NULL DEFAULT 1 COMMENT 'Artifact模式：1控制面存储 2客户存储 3离线包', customer_storage_ref VARCHAR(500) DEFAULT NULL COMMENT '客户存储Secret引用，不保存访问密钥', allowed_app_ids JSON NOT NULL COMMENT '允许构建的应用ID JSON数组', certificate_serial VARCHAR(128) DEFAULT NULL COMMENT '当前客户端证书序列号', last_nonce VARCHAR(128) DEFAULT NULL COMMENT '最近接受的请求Nonce，用于防重放', last_request_at DATETIME(3) DEFAULT NULL COMMENT '最近接受的请求时间，用于拒绝乱序重放', last_heartbeat_at DATETIME(3) DEFAULT NULL COMMENT '最近心跳时间', create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_local_agent_tenant_code (tenant_id,agent_code) COMMENT '租户内Agent编码唯一', KEY idx_local_agent_status_heartbeat (status,last_heartbeat_at) COMMENT 'Agent状态心跳扫描索引', KEY idx_local_agent_pool_drain (tenant_id,pool_code,drain_status,status) COMMENT '租户构建池可调度Agent索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 客户网络Local Builder Agent';

CREATE TABLE IF NOT EXISTS t_local_agent_registration (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '一次性注册记录ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', agent_id BIGINT NOT NULL COMMENT '关联Local Agent ID', token_hash CHAR(64) NOT NULL COMMENT '一次性注册码SHA-256摘要', status TINYINT NOT NULL DEFAULT 1 COMMENT '注册码状态：1待使用 2已使用 3已过期 4已吊销', expires_at DATETIME(3) NOT NULL COMMENT '注册码过期时间', used_at DATETIME(3) DEFAULT NULL COMMENT '首次成功注册时间', used_ip VARCHAR(64) DEFAULT NULL COMMENT '首次成功注册来源IP', create_by BIGINT NOT NULL DEFAULT 0 COMMENT '创建人ID', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_local_agent_registration_hash (token_hash) COMMENT '注册码摘要全局唯一', KEY idx_local_agent_registration_agent (tenant_id,agent_id,status,expires_at) COMMENT 'Agent有效注册码查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 Local Agent一次性注册凭证';

CREATE TABLE IF NOT EXISTS t_local_agent_certificate (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Agent客户端证书记录ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', agent_id BIGINT NOT NULL COMMENT '关联Local Agent ID', serial_number VARCHAR(128) NOT NULL COMMENT 'X.509证书序列号十六进制', fingerprint_sha256 CHAR(64) NOT NULL COMMENT '证书DER的SHA-256指纹', certificate_pem TEXT NOT NULL COMMENT '公开客户端证书PEM，不包含私钥', status TINYINT NOT NULL DEFAULT 1 COMMENT '证书状态：1有效 2已轮换 3已吊销 4已过期', not_before DATETIME(3) NOT NULL COMMENT '证书生效时间', not_after DATETIME(3) NOT NULL COMMENT '证书失效时间', revoked_at DATETIME(3) DEFAULT NULL COMMENT '证书吊销时间', revoke_reason VARCHAR(500) DEFAULT NULL COMMENT '证书吊销原因', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_local_agent_certificate_serial (serial_number) COMMENT '证书序列号唯一', UNIQUE KEY uk_local_agent_certificate_fingerprint (fingerprint_sha256) COMMENT '证书指纹唯一', KEY idx_local_agent_certificate_agent (tenant_id,agent_id,status,not_after) COMMENT 'Agent有效证书查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 Local Agent公开客户端证书与吊销状态';

CREATE TABLE IF NOT EXISTS t_local_agent_capability (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT 'Agent能力记录ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', agent_id BIGINT NOT NULL COMMENT '关联Local Agent ID', capability_key VARCHAR(128) NOT NULL COMMENT '预定义能力键，不允许任意命令', capability_value VARCHAR(500) NOT NULL COMMENT '能力版本或约束值', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_local_agent_capability (agent_id,capability_key) COMMENT 'Agent能力键唯一', KEY idx_local_agent_capability_tenant (tenant_id,capability_key) COMMENT '租户能力匹配索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 Local Agent预定义能力';

CREATE TABLE IF NOT EXISTS t_hybrid_artifact_reference (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '混合部署Artifact引用ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', agent_id BIGINT NOT NULL COMMENT '关联Local Agent ID', task_id BIGINT NOT NULL COMMENT '关联构建任务ID', builder_attempt INT NOT NULL COMMENT '构建任务fencing尝试次数', artifact_type TINYINT NOT NULL COMMENT 'Artifact类型：1源APK 2构建APK 3构建日志 4离线任务包', storage_mode TINYINT NOT NULL COMMENT '存储模式：1控制面存储 2客户存储 3离线包', object_reference VARCHAR(1000) NOT NULL COMMENT '对象引用或离线包ID，不包含访问凭证', sha256 CHAR(64) NOT NULL COMMENT 'Artifact内容SHA-256', size_bytes BIGINT NOT NULL COMMENT 'Artifact大小字节数', status TINYINT NOT NULL DEFAULT 1 COMMENT '引用状态：1待校验 2已验证 3已失效', verified_at DATETIME(3) DEFAULT NULL COMMENT '大小、SHA、租户和attempt验证时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_hybrid_artifact_attempt_type (tenant_id,task_id,builder_attempt,artifact_type) COMMENT '任务attempt内Artifact类型唯一', KEY idx_hybrid_artifact_agent (tenant_id,agent_id,status,create_time) COMMENT 'Agent Artifact引用查询索引', KEY idx_hybrid_artifact_sha (sha256,size_bytes) COMMENT 'Artifact完整性追溯索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V7 Hybrid和离线Artifact授权引用';

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_108_v7_enterprise','V7 Local Builder Agent注册证书能力与混合Artifact模型')
ON DUPLICATE KEY UPDATE description=VALUES(description);
