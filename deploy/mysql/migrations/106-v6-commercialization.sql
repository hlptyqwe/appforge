-- AppForge V6 商业化：套餐、订阅、权益、额度、用量、账单与支付。
SET NAMES utf8mb4;
SET time_zone = '+08:00';

CREATE TABLE IF NOT EXISTS t_billing_plan (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '套餐版本ID', plan_code VARCHAR(64) NOT NULL COMMENT '套餐稳定编码', plan_name VARCHAR(128) NOT NULL COMMENT '套餐展示名称', billing_cycle TINYINT NOT NULL COMMENT '计费周期：1月付 2年付',
  price_amount BIGINT NOT NULL DEFAULT 0 COMMENT '套餐价格，最小货币单位整数', currency CHAR(3) NOT NULL DEFAULT 'CNY' COMMENT 'ISO 4217大写币种', feature_json JSON NOT NULL COMMENT '非额度型功能开关JSON',
  builds_per_cycle BIGINT NOT NULL DEFAULT 0 COMMENT '每周期可计费构建次数，-1不限量', max_build_concurrency INT NOT NULL DEFAULT 1 COMMENT '租户最大构建并发，-1不限量', storage_bytes BIGINT NOT NULL DEFAULT 0 COMMENT '存储额度字节数，-1不限量',
  max_upload_bytes BIGINT NOT NULL DEFAULT 0 COMMENT '单文件上传上限字节数，-1不限量', team_seats INT NOT NULL DEFAULT 1 COMMENT '团队活跃席位数，-1不限量', api_rate_limit INT NOT NULL DEFAULT 60 COMMENT 'Open API每分钟请求上限，-1不限量',
  charge_failed_build TINYINT NOT NULL DEFAULT 0 COMMENT '失败构建计量规则：0不计费 1计费', charge_cache_hit TINYINT NOT NULL DEFAULT 1 COMMENT '缓存命中构建计量规则：0不计费 1计费', charge_retry_build TINYINT NOT NULL DEFAULT 1 COMMENT '重试构建计量规则：0不计费 1计费',
  status TINYINT NOT NULL DEFAULT 1 COMMENT '套餐版本状态：1启用 2退役', version INT NOT NULL COMMENT '套餐不可变版本号', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间，仅允许状态变化',
  PRIMARY KEY (id), UNIQUE KEY uk_billing_plan_code_version (plan_code,version) COMMENT '套餐编码版本唯一', KEY idx_billing_plan_status_cycle (status,billing_cycle,price_amount) COMMENT '可售套餐查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 不可变商业化套餐版本';

CREATE TABLE IF NOT EXISTS t_tenant_subscription (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '租户订阅ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', plan_id BIGINT NOT NULL COMMENT '当前套餐版本ID', plan_version INT NOT NULL COMMENT '订阅时固化的套餐版本号', status TINYINT NOT NULL COMMENT '订阅状态：1生效 2逾期 3宽限 4暂停 5已取消 6待支付', source TINYINT NOT NULL COMMENT '订阅来源：1Stripe 2人工合同 3平台赠送',
  external_customer_id VARCHAR(255) DEFAULT NULL COMMENT '支付提供商客户ID', external_subscription_id VARCHAR(255) DEFAULT NULL COMMENT '支付提供商订阅ID', current_period_start DATETIME(3) NOT NULL COMMENT '当前计费周期开始时间', current_period_end DATETIME(3) NOT NULL COMMENT '当前计费周期结束时间', cancel_at_period_end TINYINT NOT NULL DEFAULT 0 COMMENT '周期末取消：0否 1是', grace_until DATETIME(3) DEFAULT NULL COMMENT '宽限期截止时间',
  pending_plan_id BIGINT NOT NULL DEFAULT 0 COMMENT '周期末待切换套餐版本ID', pending_plan_version INT NOT NULL DEFAULT 0 COMMENT '周期末待切换套餐版本号', last_provider_event_at DATETIME(3) DEFAULT NULL COMMENT '最近已应用支付事件的供应商创建时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_tenant_subscription_tenant (tenant_id) COMMENT '每个租户一条当前订阅', UNIQUE KEY uk_tenant_subscription_external (source,external_subscription_id) COMMENT '供应商订阅ID唯一', KEY idx_tenant_subscription_status_period (status,current_period_end,grace_until) COMMENT '订阅到期与宽限扫描索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 租户当前订阅';

CREATE TABLE IF NOT EXISTS t_tenant_entitlement (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '租户权益快照ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', source_type TINYINT NOT NULL COMMENT '权益来源：1套餐 2人工合同 3平台赠送', source_id BIGINT NOT NULL COMMENT '来源订阅或人工授权ID', plan_id BIGINT NOT NULL COMMENT '权益基础套餐版本ID', plan_version INT NOT NULL COMMENT '权益基础套餐版本号',
  builds_per_cycle BIGINT NOT NULL COMMENT '每周期可计费构建次数，-1不限量', max_build_concurrency INT NOT NULL COMMENT '租户最大构建并发，-1不限量', storage_bytes BIGINT NOT NULL COMMENT '存储额度字节数，-1不限量', max_upload_bytes BIGINT NOT NULL COMMENT '单文件上传上限字节数，-1不限量', team_seats INT NOT NULL COMMENT '团队活跃席位数，-1不限量', api_rate_limit INT NOT NULL COMMENT 'Open API每分钟请求上限，-1不限量',
  charge_failed_build TINYINT NOT NULL COMMENT '失败构建计量规则：0不计费 1计费', charge_cache_hit TINYINT NOT NULL COMMENT '缓存命中构建计量规则：0不计费 1计费', charge_retry_build TINYINT NOT NULL COMMENT '重试构建计量规则：0不计费 1计费', override_json JSON DEFAULT NULL COMMENT '人工临时额度和功能覆盖JSON', valid_from DATETIME(3) NOT NULL COMMENT '权益生效时间', valid_until DATETIME(3) NOT NULL COMMENT '权益失效时间', status TINYINT NOT NULL DEFAULT 1 COMMENT '权益状态：1生效 2暂停', revision BIGINT NOT NULL DEFAULT 1 COMMENT '权益快照修订号',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间', PRIMARY KEY (id), UNIQUE KEY uk_tenant_entitlement_tenant (tenant_id) COMMENT '每个租户一条当前权益快照', KEY idx_tenant_entitlement_status_validity (status,valid_until) COMMENT '权益状态与有效期查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 租户当前权益快照';

CREATE TABLE IF NOT EXISTS t_usage_ledger (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '用量账本ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', metric VARCHAR(64) NOT NULL COMMENT '计量指标枚举字符串', quantity BIGINT NOT NULL COMMENT '本次用量增量，调整账可为负数', resource_type VARCHAR(64) NOT NULL COMMENT '来源资源类型', resource_id BIGINT NOT NULL DEFAULT 0 COMMENT '来源资源ID', idempotency_key VARCHAR(191) NOT NULL COMMENT '指标内幂等键', occurred_at DATETIME(3) NOT NULL COMMENT '业务发生时间', period_key CHAR(7) NOT NULL COMMENT '归属周期键YYYY-MM', metadata JSON DEFAULT NULL COMMENT '计量规则、缓存、重试或调整原因JSON', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id), UNIQUE KEY uk_usage_ledger_idempotency (tenant_id,metric,idempotency_key) COMMENT '租户指标幂等键唯一', KEY idx_usage_ledger_period_metric (tenant_id,period_key,metric,occurred_at) COMMENT '租户周期指标汇总索引', KEY idx_usage_ledger_resource (resource_type,resource_id,metric) COMMENT '资源计量追溯索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 不可变用量账本';

CREATE TABLE IF NOT EXISTS t_quota_reservation (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '额度预占ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', metric VARCHAR(64) NOT NULL COMMENT '预占额度指标枚举字符串', quantity BIGINT NOT NULL COMMENT '预占数量，必须大于0', resource_type VARCHAR(64) NOT NULL COMMENT '预占资源类型', resource_id BIGINT NOT NULL DEFAULT 0 COMMENT '预占资源ID，创建前可为0', idempotency_key VARCHAR(191) NOT NULL COMMENT '预占幂等键', period_key CHAR(7) NOT NULL COMMENT '归属周期键YYYY-MM', status TINYINT NOT NULL DEFAULT 1 COMMENT '预占状态：1预占 2确认 3释放 4过期', expires_at DATETIME(3) NOT NULL COMMENT '未确认预占过期时间', confirmed_at DATETIME(3) DEFAULT NULL COMMENT '确认时间', released_at DATETIME(3) DEFAULT NULL COMMENT '释放或过期时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_quota_reservation_idempotency (tenant_id,metric,idempotency_key) COMMENT '租户指标预占幂等键唯一', KEY idx_quota_reservation_active (tenant_id,metric,period_key,status,expires_at) COMMENT '生效预占汇总索引', KEY idx_quota_reservation_resource (resource_type,resource_id,status) COMMENT '资源预占确认释放索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 并发安全额度预占';

CREATE TABLE IF NOT EXISTS t_usage_threshold_notification (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '阈值通知ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', metric VARCHAR(64) NOT NULL COMMENT '用量指标枚举字符串', period_key CHAR(7) NOT NULL COMMENT '归属周期键YYYY-MM', threshold_percent INT NOT NULL COMMENT '触发阈值百分比：70、90或100', usage_quantity BIGINT NOT NULL COMMENT '触发时当前用量', limit_quantity BIGINT NOT NULL COMMENT '触发时权益限额', status TINYINT NOT NULL DEFAULT 1 COMMENT '通知状态：1待发送 2已发送 3发送失败', outbox_event_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联quota Webhook Outbox事件ID', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_usage_threshold_once (tenant_id,metric,period_key,threshold_percent) COMMENT '租户周期指标阈值仅通知一次', KEY idx_usage_threshold_status (status,create_time) COMMENT '通知发送状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 用量阈值幂等通知';

CREATE TABLE IF NOT EXISTS t_invoice (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '账单ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', subscription_id BIGINT NOT NULL COMMENT '关联订阅ID', invoice_no VARCHAR(64) NOT NULL COMMENT '平台账单号', external_invoice_id VARCHAR(255) DEFAULT NULL COMMENT '支付提供商账单ID', status TINYINT NOT NULL COMMENT '账单状态：1草稿 2待支付 3已支付 4支付失败 5已作废 6已退款', currency CHAR(3) NOT NULL COMMENT 'ISO 4217大写币种', subtotal_amount BIGINT NOT NULL COMMENT '税前小计整数', discount_amount BIGINT NOT NULL DEFAULT 0 COMMENT '折扣金额整数', tax_amount BIGINT NOT NULL DEFAULT 0 COMMENT '税费整数', total_amount BIGINT NOT NULL COMMENT '应付总额整数', paid_amount BIGINT NOT NULL DEFAULT 0 COMMENT '已支付金额整数', refunded_amount BIGINT NOT NULL DEFAULT 0 COMMENT '已退款金额整数', period_start DATETIME(3) NOT NULL COMMENT '账单周期开始时间', period_end DATETIME(3) NOT NULL COMMENT '账单周期结束时间', due_at DATETIME(3) DEFAULT NULL COMMENT '最晚付款时间', paid_at DATETIME(3) DEFAULT NULL COMMENT '支付完成时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '状态更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_invoice_no (invoice_no) COMMENT '平台账单号唯一', UNIQUE KEY uk_invoice_external (external_invoice_id) COMMENT '供应商账单ID唯一', KEY idx_invoice_tenant_time (tenant_id,create_time) COMMENT '租户账单列表索引', KEY idx_invoice_status_due (status,due_at) COMMENT '待支付账单扫描索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 不可变账单';

CREATE TABLE IF NOT EXISTS t_invoice_item (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '账单项ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', invoice_id BIGINT NOT NULL COMMENT '所属账单ID', line_key VARCHAR(191) NOT NULL COMMENT '账单内不可变行幂等键', item_type TINYINT NOT NULL COMMENT '账单项类型：1套餐 2用量 3折扣 4税费 5调整 6退款', description VARCHAR(500) NOT NULL COMMENT '账单项说明', metric VARCHAR(64) DEFAULT NULL COMMENT '用量指标', quantity BIGINT NOT NULL DEFAULT 1 COMMENT '计费数量整数', unit_amount BIGINT NOT NULL COMMENT '单价整数', amount BIGINT NOT NULL COMMENT '行金额整数', metadata JSON DEFAULT NULL COMMENT '套餐版本或账本范围JSON', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id), UNIQUE KEY uk_invoice_item_line (invoice_id,line_key) COMMENT '账单内行幂等键唯一', KEY idx_invoice_item_tenant_invoice (tenant_id,invoice_id,id) COMMENT '租户账单项查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 不可变账单项';

CREATE TABLE IF NOT EXISTS t_payment_transaction (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '支付交易ID', tenant_id BIGINT NOT NULL COMMENT '所属租户ID', invoice_id BIGINT NOT NULL DEFAULT 0 COMMENT '关联账单ID', provider VARCHAR(32) NOT NULL COMMENT '支付提供商：stripe/manual', provider_transaction_id VARCHAR(255) NOT NULL COMMENT '供应商交易ID', transaction_type TINYINT NOT NULL COMMENT '交易类型：1扣款 2退款 3争议', status TINYINT NOT NULL COMMENT '交易状态：1处理中 2成功 3失败 4已撤销', currency CHAR(3) NOT NULL COMMENT 'ISO 4217大写币种', amount BIGINT NOT NULL COMMENT '交易金额整数', failure_code VARCHAR(64) DEFAULT NULL COMMENT '失败结构化代码', failure_message VARCHAR(500) DEFAULT NULL COMMENT '失败脱敏摘要', occurred_at DATETIME(3) NOT NULL COMMENT '供应商业务发生时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '状态更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_payment_provider_transaction (provider,provider_transaction_id,transaction_type) COMMENT '供应商交易类型幂等唯一', KEY idx_payment_tenant_time (tenant_id,occurred_at) COMMENT '租户支付流水查询索引', KEY idx_payment_invoice_status (invoice_id,status) COMMENT '账单支付状态查询索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 支付交易流水';

CREATE TABLE IF NOT EXISTS t_billing_webhook_event (
  id BIGINT NOT NULL AUTO_INCREMENT COMMENT '支付回调事件ID', provider VARCHAR(32) NOT NULL COMMENT '支付提供商：stripe', provider_event_id VARCHAR(255) NOT NULL COMMENT '供应商事件ID', event_type VARCHAR(128) NOT NULL COMMENT '供应商事件类型', event_created_at DATETIME(3) NOT NULL COMMENT '供应商事件创建时间', payload_sha256 CHAR(64) NOT NULL COMMENT '原始请求体SHA-256', payload_ciphertext MEDIUMTEXT NOT NULL COMMENT '原始请求体secretbox密文', status TINYINT NOT NULL DEFAULT 1 COMMENT '处理状态：1待处理 2已应用 3已忽略 4失败', attempt INT NOT NULL DEFAULT 0 COMMENT '处理尝试次数', tenant_id BIGINT NOT NULL DEFAULT 0 COMMENT '解析后的租户ID', error_message VARCHAR(1000) DEFAULT NULL COMMENT '处理失败脱敏摘要', processed_at DATETIME(3) DEFAULT NULL COMMENT '处理完成时间', retain_until DATETIME(3) NOT NULL COMMENT '密文载荷保留截止时间', create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间', update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id), UNIQUE KEY uk_billing_webhook_provider_event (provider,provider_event_id) COMMENT '供应商事件幂等唯一', KEY idx_billing_webhook_status (status,event_created_at,id) COMMENT '支付事件处理索引', KEY idx_billing_webhook_retention (retain_until,status) COMMENT '支付载荷保留清理索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='V6 支付回调可靠事件';

INSERT INTO t_billing_plan (plan_code,plan_name,billing_cycle,price_amount,currency,feature_json,builds_per_cycle,max_build_concurrency,storage_bytes,max_upload_bytes,team_seats,api_rate_limit,charge_failed_build,charge_cache_hit,charge_retry_build,status,version)
VALUES
  ('free','免费版',1,0,'CNY',JSON_OBJECT('webhook',true,'sourceIntegration',false),50,1,1073741824,268435456,3,60,0,1,1,1,1),
  ('pro','专业版',1,19900,'CNY',JSON_OBJECT('webhook',true,'sourceIntegration',true),1000,3,53687091200,2147483648,20,600,0,1,1,1,1),
  ('business','企业版',2,199900,'CNY',JSON_OBJECT('webhook',true,'sourceIntegration',true,'prioritySupport',true),-1,10,536870912000,5368709120,100,3000,0,0,0,1,1)
ON DUPLICATE KEY UPDATE plan_code=VALUES(plan_code);

INSERT IGNORE INTO t_tenant_subscription (tenant_id,plan_id,plan_version,status,source,current_period_start,current_period_end,cancel_at_period_end)
SELECT t.id,p.id,p.version,1,3,DATE_FORMAT(CURRENT_DATE,'%Y-%m-01'),DATE_ADD(DATE_FORMAT(CURRENT_DATE,'%Y-%m-01'),INTERVAL 1 MONTH),0
FROM sys_tenant t JOIN t_billing_plan p ON p.plan_code='free' AND p.version=1;

INSERT IGNORE INTO t_tenant_entitlement (tenant_id,source_type,source_id,plan_id,plan_version,builds_per_cycle,max_build_concurrency,storage_bytes,max_upload_bytes,team_seats,api_rate_limit,charge_failed_build,charge_cache_hit,charge_retry_build,valid_from,valid_until,status,revision)
SELECT s.tenant_id,3,s.id,p.id,p.version,p.builds_per_cycle,p.max_build_concurrency,p.storage_bytes,p.max_upload_bytes,p.team_seats,p.api_rate_limit,p.charge_failed_build,p.charge_cache_hit,p.charge_retry_build,s.current_period_start,s.current_period_end,1,1
FROM t_tenant_subscription s JOIN t_billing_plan p ON p.id=s.plan_id;

-- 将升级前仍占用的私有对象和活跃成员写入不可变基线账，避免迁移后额度被低估。
INSERT IGNORE INTO t_usage_ledger
(tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,occurred_at,period_key,metadata)
SELECT o.tenant_id,
  CASE WHEN o.object_type IN (3,8) THEN 'storage.artifact_bytes'
       WHEN o.object_type=4 THEN 'storage.log_bytes' ELSE 'storage.source_bytes' END,
  o.size_bytes,'storage',o.id,CONCAT('v6-backfill-storage:',o.id),o.create_time,
  DATE_FORMAT(o.create_time,'%Y-%m'),JSON_OBJECT('migration','v6','objectType',o.object_type)
FROM t_storage_object o WHERE o.status IN (2,3) AND o.size_bytes>0;

INSERT IGNORE INTO t_usage_ledger
(tenant_id,metric,quantity,resource_type,resource_id,idempotency_key,occurred_at,period_key,metadata)
SELECT u.tenant_id,'team.active_seats',1,'system_user',u.id,CONCAT('v6-backfill-seat:',u.id),
  CURRENT_TIMESTAMP(3),DATE_FORMAT(CURRENT_DATE,'%Y-%m'),JSON_OBJECT('migration','v6')
FROM sys_user u WHERE u.tenant_id>0 AND u.enabled=1;

INSERT INTO sys_schema_migration (version,description)
VALUES ('20260814_106_v6_commercialization','V6套餐订阅权益用量额度账单与支付模型')
ON DUPLICATE KEY UPDATE description=VALUES(description);
