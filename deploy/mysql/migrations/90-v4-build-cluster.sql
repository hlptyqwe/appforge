-- AppForge V4 Builder集群、公平调度、并发槽位、构建缓存和调度事件迁移。
-- 本文件可重复执行，并兼容已有开发数据卷。
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS t_builder_node (
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

CREATE TABLE IF NOT EXISTS t_build_concurrency_policy (
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

CREATE TABLE IF NOT EXISTS t_build_fair_queue (
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

CREATE TABLE IF NOT EXISTS t_build_slot_lease (
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

CREATE TABLE IF NOT EXISTS t_build_cache_entry (
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

CREATE TABLE IF NOT EXISTS t_build_scheduler_event (
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

DROP PROCEDURE IF EXISTS appforge_apply_v4_build_cluster;
DELIMITER //
CREATE PROCEDURE appforge_apply_v4_build_cluster()
BEGIN
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'pool_code') THEN
    ALTER TABLE t_build_task ADD COLUMN pool_code VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT 'V4任务目标构建池编码' AFTER template_snapshot;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'cache_key') THEN
    ALTER TABLE t_build_task ADD COLUMN cache_key CHAR(64) DEFAULT NULL COMMENT 'V4不可变输入构建缓存键' AFTER pool_code;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'cache_entry_id') THEN
    ALTER TABLE t_build_task ADD COLUMN cache_entry_id BIGINT NOT NULL DEFAULT 0 COMMENT 'V4命中的构建缓存条目ID' AFTER cache_key;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'cache_hit') THEN
    ALTER TABLE t_build_task ADD COLUMN cache_hit TINYINT NOT NULL DEFAULT 0 COMMENT 'V4缓存命中标记：0未命中 1命中' AFTER cache_entry_id;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'cancel_requested_at') THEN
    ALTER TABLE t_build_task ADD COLUMN cancel_requested_at DATETIME(3) DEFAULT NULL COMMENT '取消请求时间' AFTER lease_until;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'cancelled_at') THEN
    ALTER TABLE t_build_task ADD COLUMN cancelled_at DATETIME(3) DEFAULT NULL COMMENT '任务取消完成时间' AFTER cancel_requested_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'cancel_reason') THEN
    ALTER TABLE t_build_task ADD COLUMN cancel_reason VARCHAR(500) DEFAULT NULL COMMENT '任务取消原因' AFTER cancelled_at;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND COLUMN_NAME = 'retry_of_task_id') THEN
    ALTER TABLE t_build_task ADD COLUMN retry_of_task_id BIGINT NOT NULL DEFAULT 0 COMMENT '重试来源任务ID，0表示非重试任务' AFTER cancel_reason;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND INDEX_NAME = 'idx_build_pool_queue') THEN
    ALTER TABLE t_build_task ADD KEY idx_build_pool_queue (pool_code, status, priority, queued_at, id) COMMENT 'V4构建池调度队列索引';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND INDEX_NAME = 'idx_build_cache_key') THEN
    ALTER TABLE t_build_task ADD KEY idx_build_cache_key (tenant_id, cache_key, status) COMMENT 'V4任务缓存键查询索引';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 't_build_task' AND INDEX_NAME = 'idx_build_retry_source') THEN
    ALTER TABLE t_build_task ADD KEY idx_build_retry_source (retry_of_task_id, create_time) COMMENT 'V4任务重试链查询索引';
  END IF;
END//
DELIMITER ;

CALL appforge_apply_v4_build_cluster();
DROP PROCEDURE IF EXISTS appforge_apply_v4_build_cluster;

INSERT INTO t_build_concurrency_policy (
  tenant_id, app_id, pool_code, max_concurrency, fair_weight, max_priority, status, create_by
) VALUES (0, 0, 'default', 10, 100, 100, 1, 0)
ON DUPLICATE KEY UPDATE
  max_concurrency = VALUES(max_concurrency), fair_weight = VALUES(fair_weight),
  max_priority = VALUES(max_priority), status = VALUES(status);

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260814_90_v4_build_cluster', 'V4 Builder集群、公平调度、并发槽位、构建缓存与调度事件')
ON DUPLICATE KEY UPDATE description = VALUES(description);
