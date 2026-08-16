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

-- AIR_GAPPED任务包包含源APK与签名材料，导出、导入和查询必须分别授权。
-- 本迁移只登记权限目录，不向任何已有角色写入sys_role_menu，避免升级时扩大权限。
INSERT INTO sys_menu (
  id,parent_id,app_scope,name,menu_type,method,path,component,perms,
  icon,sort,visible,enabled,create_times,update_times
) VALUES
  (4515,4500,1,'导出断网任务包',3,'POST','/core/enterprise/air-gapped/exports','','enterprise:air-gapped:export','',6,2,1,0,0),
  (4516,4500,1,'导入断网结果包',3,'POST','/core/enterprise/air-gapped/imports','','enterprise:air-gapped:import','',7,2,1,0,0),
  (4517,4500,1,'查看断网任务包',3,'GET','/core/enterprise/air-gapped/packages/:packageCode','','enterprise:air-gapped:view','',8,2,1,0,0),
  (4615,4600,2,'导出断网任务包',3,'POST','/core/enterprise/air-gapped/exports','','enterprise:air-gapped:export','',6,2,1,0,0),
  (4616,4600,2,'导入断网结果包',3,'POST','/core/enterprise/air-gapped/imports','','enterprise:air-gapped:import','',7,2,1,0,0),
  (4617,4600,2,'查看断网任务包',3,'GET','/core/enterprise/air-gapped/packages/:packageCode','','enterprise:air-gapped:view','',8,2,1,0,0)
ON DUPLICATE KEY UPDATE name=VALUES(name),parent_id=VALUES(parent_id),app_scope=VALUES(app_scope),method=VALUES(method),
  path=VALUES(path),component=VALUES(component),perms=VALUES(perms),icon=VALUES(icon),sort=VALUES(sort),visible=VALUES(visible),enabled=VALUES(enabled);

INSERT INTO sys_schema_migration (version, description)
VALUES ('20260815_113_v7_air_gapped', 'V7 AIR_GAPPED离线任务结果双向签名与防重放状态')
ON DUPLICATE KEY UPDATE description=VALUES(description);
